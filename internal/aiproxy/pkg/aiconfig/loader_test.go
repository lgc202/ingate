package aiconfig

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
)

type scriptedADS struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	run func(discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error
}

// feedbackClient 只截取客户端提交给 gRPC 的消息，不把 Send 成功冒充远端必然收到。
type feedbackClient struct {
	discoveryv3.AggregatedDiscoveryServiceClient
	stream *feedbackStream
}

type feedbackStream struct {
	grpc.ClientStream
	responses []*discoveryv3.DeltaDiscoveryResponse
	feedback  []*discoveryv3.DeltaDiscoveryRequest
}

func TestDeltaStreamSubmitsNACKBeforeReconnect(t *testing.T) {
	entry, rule := testResources()
	bad := testBatch(t)
	bad.Nonce = "bad-batch"
	bad.RemovedResources = []string{entry.Name}
	stream := &feedbackStream{responses: []*discoveryv3.DeltaDiscoveryResponse{testBatch(t, entry, rule), bad}}
	loader := NewLoader(&feedbackClient{stream: stream}, "test", slog.Default())

	accepted, err := loader.runStream(t.Context())

	if !accepted || err == nil || len(stream.feedback) != 3 {
		t.Fatalf("runStream(bad second batch) = (%t, %v), feedback count %d, want subscribe, ACK, NACK and reconnect failure", accepted, err, len(stream.feedback))
	}

	nack := stream.feedback[2]
	if nack.ResponseNonce != bad.Nonce || nack.ErrorDetail == nil || nack.ErrorDetail.Code == 0 || len(nack.InitialResourceVersions) != 0 {
		t.Errorf("submitted NACK = %v, want rejected nonce and error without new accepted versions", nack)
	}
}

func TestDeltaStreamAppliesBeforeACKAndNACKKeepsAcceptedVersions(t *testing.T) {
	entry, rule := testResources()
	initial := testBatch(t, entry, rule)
	initial.Nonce = "accepted"
	bad := testBatch(t)
	bad.Nonce, bad.RemovedResources = "rejected", []string{entry.Name}
	resumed := testBatch(t)
	versions := make(map[string]string)
	for _, resource := range initial.Resources {
		versions[resource.Name] = resource.Version
	}

	var attempts atomic.Int32
	var loader *Loader
	client := newScriptedADSClient(t, func(stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
		request, err := stream.Recv()
		if err != nil {
			return err
		}

		if request.TypeUrl != aidataplanev1.ResourceType || request.GetNode().GetCluster() != aidataplanev1.SharedDataPlaneCluster ||
			request.GetNode().GetId() != "test-node" || len(request.ResourceNamesSubscribe) != 1 || request.ResourceNamesSubscribe[0] != "*" || request.ResponseNonce != "" {
			t.Errorf("initial Delta request = %v, want scoped wildcard subscription with node and no inherited nonce", request)
		}

		if attempts.Add(1) == 1 {
			if len(request.InitialResourceVersions) != 0 {
				t.Error("first subscription carried unaccepted versions")
			}

			if err := stream.Send(initial); err != nil {
				return err
			}

			ack, err := stream.Recv()
			if err != nil {
				return err
			}

			view, loaded := loader.Capture()
			if !loaded {
				t.Error("received ACK before publishing a complete runtime view")
				return errors.New("missing runtime view")
			}

			if ack.ResponseNonce != initial.Nonce || ack.ErrorDetail != nil || len(ack.InitialResourceVersions) != 0 {
				t.Errorf("ACK = %v, want matching nonce without error or initial versions", ack)
			}

			if _, err := view.ResolveModel(8080, "host", "coding"); err != nil {
				t.Errorf("ResolveModel(before ACK) error = %v", err)
			}

			if err := stream.Send(bad); err != nil {
				return err
			}

			nack, err := stream.Recv()
			if err != nil {
				return err
			}

			if nack.ResponseNonce != bad.Nonce || nack.ErrorDetail == nil || nack.ErrorDetail.Code == 0 {
				t.Errorf("NACK = %v, want rejected nonce and an error code", nack)
			}

			if current, loaded := loader.Capture(); !loaded || current != view {
				t.Error("NACK changed the active runtime view")
			}

			return nil
		}

		if !maps.Equal(request.InitialResourceVersions, versions) {
			t.Errorf("reconnect versions = %v, want last accepted %v", request.InitialResourceVersions, versions)
		}

		if err := stream.Send(resumed); err != nil {
			return err
		}

		_, err = stream.Recv()

		return err
	})
	loader = NewLoader(client, "test-node", slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	accepted, err := loader.runStream(ctx)
	if !accepted || err == nil {
		t.Fatalf("runStream(rejected second batch) = (%t, %v), want prior accepted state and failure", accepted, err)
	}

	accepted, err = loader.runStream(ctx)
	if !accepted || !errors.Is(err, io.EOF) {
		t.Errorf("runStream(known-version resume) = (%t, %v), want valid empty difference followed by EOF", accepted, err)
	}
}

func TestLoaderRetriesWithBackoffAndStops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var attempts atomic.Int32
		client := newScriptedADSClient(t, func(stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
			if _, err := stream.Recv(); err != nil {
				return err
			}

			attempts.Add(1)

			return nil
		})
		loader := NewLoader(client, "test-node", slog.New(slog.NewTextHandler(io.Discard, nil)))
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- loader.Run(ctx) }()
		synctest.Wait()
		if got := attempts.Load(); got != 1 {
			t.Fatalf("Run(immediate failure) attempts = %d, want one before backoff", got)
		}

		time.Sleep(2 * time.Second)
		synctest.Wait()
		if got := attempts.Load(); got < 2 || got > 3 {
			t.Errorf("Run(after backoff) attempts = %d, want bounded retries", got)
		}

		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run(cancelled) error = %v, want context.Canceled", err)
		}

		if err := loader.Run(t.Context()); err == nil {
			t.Error("Run(second invocation) error = nil, want single-use failure")
		}
	})
}

func TestLoaderKeepsConfigurationAcrossDisconnectAndClearsOnStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entry, rule := testResources()
		initial := testBatch(t, entry, rule)
		var attempts atomic.Int32
		client := newScriptedADSClient(t, func(stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
			if _, err := stream.Recv(); err != nil {
				return err
			}

			if attempts.Add(1) == 1 {
				if err := stream.Send(initial); err != nil {
					return err
				}

				_, err := stream.Recv()

				return err
			}

			<-stream.Context().Done()

			return stream.Context().Err()
		})
		loader := NewLoader(client, "test-node", slog.New(slog.NewTextHandler(io.Discard, nil)))
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- loader.Run(ctx) }()
		synctest.Wait()
		previous, loaded := loader.Capture()
		if !loaded {
			t.Fatal("Capture(after first response) has no configuration")
		}

		time.Sleep(2 * time.Second)
		synctest.Wait()
		if current, loaded := loader.Capture(); !loaded || current != previous || attempts.Load() != 2 {
			t.Error("disconnect/reconnect discarded the last valid view or did not reconnect")
		}

		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run(cancelled open stream) error = %v", err)
		}

		if _, loaded := loader.Capture(); loaded || len(loader.state.resources) != 0 {
			t.Error("Run(stop) retained the current pointer or owned resources")
		}

		if _, err := previous.ResolveModel(8080, "host", "coding"); err != nil {
			t.Errorf("ResolveModel(captured before stop) error = %v, want immutable retained view", err)
		}
	})
}

func TestLoaderRunRejectsCancelledContextWithoutDial(t *testing.T) {
	loader := NewLoader(nil, "test", slog.Default())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := loader.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Run(cancelled before subscription) error = %v, want context.Canceled", err)
	}
}

func (s *scriptedADS) DeltaAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return s.run(stream)
}

func (c *feedbackClient) DeltaAggregatedResources(context.Context, ...grpc.CallOption) (discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesClient, error) {
	return c.stream, nil
}

func (s *feedbackStream) Send(request *discoveryv3.DeltaDiscoveryRequest) error {
	s.feedback = append(s.feedback, request)

	return nil
}

func (s *feedbackStream) Recv() (*discoveryv3.DeltaDiscoveryResponse, error) {
	if len(s.responses) == 0 {
		return nil, io.EOF
	}

	response := s.responses[0]
	s.responses = s.responses[1:]

	return response, nil
}

func newScriptedADSClient(t *testing.T, run func(discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error) discoveryv3.AggregatedDiscoveryServiceClient {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(server, &scriptedADS{run: run})
	var serving sync.WaitGroup
	serving.Go(func() {
		if err := server.Serve(listener); err != nil {
			t.Errorf("Serve(scripted ADS) error = %v", err)
		}
	})
	t.Cleanup(func() { server.Stop(); serving.Wait() })
	connection, err := grpc.NewClient("passthrough:///scripted-ads",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
	)
	if err != nil {
		t.Fatalf("NewClient(scripted ADS) error = %v", err)
	}

	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Errorf("Close(scripted ADS) error = %v", err)
		}
	})

	return discoveryv3.NewAggregatedDiscoveryServiceClient(connection)
}
