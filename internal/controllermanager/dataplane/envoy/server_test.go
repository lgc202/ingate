package envoy

import (
	"context"
	"net"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

func TestServerLifecycle(t *testing.T) {
	publisher := NewPublisher()
	server := NewServer("127.0.0.1:0", publisher)
	if !server.NeedLeaderElection() {
		t.Error("NeedLeaderElection() = false, want true")
	}
	if err := server.CheckReady(nil); err == nil {
		t.Error("CheckReady(before Start) error = nil, want not listening")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- server.Start(ctx) }()

	deadline := time.After(2 * time.Second)
	for !server.listening.Load() {
		select {
		case err := <-finished:
			t.Fatalf("Start() exited before listening: %v", err)
		case <-deadline:
			t.Fatal("Start() did not begin listening within 2s")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := server.CheckReady(nil); err == nil {
		t.Error("CheckReady(before initial Publish) error = nil, want missing configuration")
	}
	if err := publisher.Publish(ctx, dataplane.Config{}); err != nil {
		t.Fatalf("Publish(empty) error = %v, want nil", err)
	}
	if err := server.CheckReady(nil); err != nil {
		t.Errorf("CheckReady(after initial Publish) error = %v, want nil", err)
	}

	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Errorf("Start(after cancellation) error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start() did not stop within 2s after cancellation")
	}
	if err := server.CheckReady(nil); err == nil {
		t.Error("CheckReady(after Stop) error = nil, want not listening")
	}
}

func TestServerStartReportsBindError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	server := NewServer(listener.Addr().String(), NewPublisher())
	if err := server.Start(context.Background()); err == nil {
		t.Error("Start(occupied address) error = nil, want bind error")
	}
}

func TestADSStreamsPublishedResources(t *testing.T) {
	publisher := NewPublisher()
	config := dataplane.Config{
		Listeners: []dataplane.Listener{{Port: 8080, Hosts: []dataplane.Host{{
			Name: "gateway/main/listener/http", Hostname: "example.com",
			Routes: []dataplane.Route{{
				Name: "api", PathType: dataplane.PathMatchPrefix, Path: "/api", BackendName: "backend",
			}},
		}}}},
		Backends: []dataplane.Backend{{
			Name: "backend", Endpoints: []dataplane.Endpoint{{Address: "192.0.2.1", Port: 9000, Weight: 1}},
		}},
	}

	if err := publisher.Publish(context.Background(), config); err != nil {
		t.Fatalf("Publish(config) error = %v, want nil", err)
	}

	listener := bufconn.Listen(1 << 20)
	server := NewServer("", publisher)
	finished := make(chan error, 1)
	go func() {
		finished <- server.grpcServer.Serve(listener)
	}()
	t.Cleanup(func() {
		server.grpcServer.Stop()
		_ = listener.Close()
		if err := <-finished; err != nil {
			t.Errorf("gRPC Serve() after Stop = %v, want nil", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient(bufconn) error = %v, want nil", err)
	}
	defer func() { _ = conn.Close() }()

	stream, err := discoveryv3.NewAggregatedDiscoveryServiceClient(conn).StreamAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("StreamAggregatedResources() error = %v, want nil", err)
	}
	for _, request := range []struct {
		typeURL string
		name    string
	}{
		{resourcev3.ClusterType, ""},
		{resourcev3.EndpointType, "backend/backend"},
		{resourcev3.ListenerType, ""},
		{resourcev3.RouteType, "route/8080"},
	} {
		message := &discoveryv3.DiscoveryRequest{
			Node:    &corev3.Node{Id: "envoy-1", Cluster: sharedDataPlaneCluster},
			TypeUrl: request.typeURL,
		}
		if request.name != "" {
			message.ResourceNames = []string{request.name}
		}
		if err := stream.Send(message); err != nil {
			t.Fatalf("ADS Send(%s) error = %v, want nil", request.typeURL, err)
		}
		response, err := stream.Recv()
		if err != nil {
			t.Fatalf("ADS Recv(%s) error = %v, want nil", request.typeURL, err)
		}
		if got := response.TypeUrl; got != request.typeURL {
			t.Errorf("ADS Recv(%s) type = %q, want %q", request.typeURL, got, request.typeURL)
		}
		if got := len(response.Resources); got != 1 {
			t.Errorf("ADS Recv(%s) resources = %d, want 1", request.typeURL, got)
		}
	}
}
