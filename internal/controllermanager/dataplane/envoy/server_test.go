package envoy

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/go-logr/logr"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

// callbackLogWriter 适配异步日志与测试断言；超时或取消后不阻塞 ADS 清理。
// 对象只用于一次 ADS 测试，Context 由该测试创建和取消，gRPC 停止后不再复用。
// io.Writer 的固定签名没有 Context 参数，因此在此保存该次测试的截止时间。
type callbackLogWriter struct {
	ctx  context.Context //nolint:containedctx // 单次 ADS 测试的 io.Writer 适配，生命周期由测试取消并等待 gRPC 退出。
	logs chan string
}

func (w callbackLogWriter) Write(data []byte) (int, error) {
	select {
	case w.logs <- string(data):
		return len(data), nil
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	}
}

func TestServerLifecycle(t *testing.T) {
	publisher := NewPublisher("", "")
	server := NewServer("127.0.0.1:0", publisher, logr.Discard())
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

	server := NewServer(listener.Addr().String(), NewPublisher("", ""), logr.Discard())
	if err := server.Start(context.Background()); err == nil {
		t.Error("Start(occupied address) error = nil, want bind error")
	}
}

func TestADSStreamsPublishedResources(t *testing.T) {
	publisher := NewPublisher("", "")
	config := dataplane.Config{
		Listeners: []dataplane.Listener{{
			Port: 8080,
			Hosts: []dataplane.Host{{
				Name:     "gateway/main/listener/http",
				Hostname: "example.com",
				Routes: []dataplane.Route{{
					Name:        "api",
					PathType:    dataplane.PathMatchPrefix,
					Path:        "/api",
					BackendName: "backend",
				}},
			}},
		}},
		Backends: []dataplane.Backend{{
			Name: "backend",
			Endpoints: []dataplane.Endpoint{{
				Address: "192.0.2.1",
				Port:    9000,
				Weight:  1,
			}},
		}},
	}

	if err := publisher.Publish(context.Background(), config); err != nil {
		t.Fatalf("Publish(config) error = %v, want nil", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	logs := make(chan string, 1)
	logger := logr.FromSlogHandler(slog.NewJSONHandler(callbackLogWriter{ctx: ctx, logs: logs}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	client := newTestADSClient(t, publisher, logger)

	stream, err := client.DeltaAggregatedResources(ctx)
	if err != nil {
		t.Fatalf("DeltaAggregatedResources() error = %v, want nil", err)
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
		message := &discoveryv3.DeltaDiscoveryRequest{
			Node:    &corev3.Node{Id: "envoy-1", Cluster: sharedDataPlaneCluster},
			TypeUrl: request.typeURL,
		}
		if request.name != "" {
			message.ResourceNamesSubscribe = []string{request.name}
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

		feedback := &discoveryv3.DeltaDiscoveryRequest{
			TypeUrl:       request.typeURL,
			ResponseNonce: response.Nonce,
		}
		if err := stream.Send(feedback); err != nil {
			t.Fatalf("ADS Send(ACK %s) = %v", request.typeURL, err)
		}

		record := receiveCallbackLog(t, ctx, logs)
		if record["result"] != "ack" || record["system_version"] != response.SystemVersionInfo ||
			record["nonce"] != response.Nonce || record["node_id"] != "envoy-1" {
			t.Errorf("ADS ACK log = %v, want current response and envoy-1", record)
		}

		if request.typeURL == resourcev3.RouteType {
			config.Listeners[0].Hosts[0].Routes[0].Path = "/v2/api"
			publishTestSnapshot(t, publisher, config)

			rejected, err := stream.Recv()
			if err != nil {
				t.Fatalf("Delta ADS Recv(before NACK) error = %v", err)
			}

			if rejected.TypeUrl != resourcev3.RouteType || rejected.Nonce == response.Nonce {
				t.Fatalf("Delta ADS(response to reject) = %v, want new RDS response", rejected)
			}

			feedback.ResponseNonce = rejected.Nonce
			feedback.ErrorDetail = &statuspb.Status{Code: 3, Message: "secret configuration"}
			if err := stream.Send(feedback); err != nil {
				t.Fatalf("ADS Send(NACK) = %v", err)
			}

			record := receiveCallbackLog(t, ctx, logs)
			if record["result"] != "nack" || record["system_version"] != rejected.SystemVersionInfo ||
				record["error_code"] != float64(3) {
				t.Errorf("ADS NACK log = %v, want rejected response and error code", record)
			}

			config.Listeners[0].Hosts[0].Routes[0].Path = "/v3/api"
			publishTestSnapshot(t, publisher, config)

			update, err := stream.Recv()
			if err != nil {
				t.Fatalf("Delta ADS Recv(after NACK) error = %v", err)
			}

			if update.TypeUrl != resourcev3.RouteType || len(update.Resources) != 1 || update.Resources[0].Name != "route/8080" {
				t.Errorf("Delta ADS(after NACK) = %v, want changed route/8080 on the same stream", update)
			}
		}
	}
}

func newTestADSClient(t *testing.T, publisher *Publisher, logger logr.Logger) discoveryv3.AggregatedDiscoveryServiceClient {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := NewServer("", publisher, logger)
	finished := make(chan error, 1)
	go func() { finished <- server.grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		server.grpcServer.Stop()
		_ = listener.Close()
		if err := <-finished; err != nil {
			t.Errorf("gRPC Serve() after Stop = %v, want nil", err)
		}
	})

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

	t.Cleanup(func() { _ = conn.Close() })

	return discoveryv3.NewAggregatedDiscoveryServiceClient(conn)
}

func receiveCallbackLog(t *testing.T, ctx context.Context, logs <-chan string) map[string]any {
	t.Helper()
	select {
	case log := <-logs:
		return readCallbackLog(t, bytes.NewBufferString(log))
	case <-ctx.Done():
		t.Fatalf("ADS feedback was not observed: %v", ctx.Err())
		return nil
	}
}
