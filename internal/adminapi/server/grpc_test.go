package server_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestCreateGatewayGRPC(t *testing.T) {
	client, backend := newTestGRPCClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.CreateGateway(ctx, &adminv1.CreateGatewayRequest{})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("CreateGateway(empty) code = %s, want InvalidArgument; error = %v", code, err)
	}
	if actions := backend.Actions(); len(actions) != 0 {
		t.Errorf("CreateGateway(empty) caused %d API Server calls, want 0", len(actions))
	}

	request := &adminv1.CreateGatewayRequest{
		Name: "public-gateway",
		Listeners: []*adminv1.Listener{{
			Name:     "http",
			Protocol: adminv1.ListenerProtocol_LISTENER_PROTOCOL_HTTP,
			Port:     8080,
		}},
	}
	reply, err := client.CreateGateway(ctx, request)
	if err != nil {
		t.Fatalf("CreateGateway(public-gateway) error = %v, want nil", err)
	}
	if name := reply.GetGateway().GetName(); name != "public-gateway" {
		t.Errorf("created Gateway name = %q, want public-gateway", name)
	}

	// Kratos v3 把 HTTP 409 映射为 gRPC Aborted；业务原因仍由 ErrorInfo 保留。
	_, err = client.CreateGateway(ctx, request)
	if code := status.Code(err); code != codes.Aborted {
		t.Errorf("CreateGateway(duplicate) code = %s, want Aborted; error = %v", code, err)
	}
	if reason := kratoserrors.Reason(err); reason != "GATEWAY_ALREADY_EXISTS" {
		t.Errorf("CreateGateway(duplicate) reason = %q, want GATEWAY_ALREADY_EXISTS", reason)
	}

	backend.PrependReactor("create", "gateways", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("backend unavailable")
	})
	request.Name = "another-gateway"
	_, err = client.CreateGateway(ctx, request)
	if code := status.Code(err); code != codes.Internal {
		t.Errorf("CreateGateway(backend failure) code = %s, want Internal; error = %v", code, err)
	}
	if message := status.Convert(err).Message(); message != "服务器内部错误" {
		t.Errorf("CreateGateway(backend failure) message = %q, want Chinese public error", message)
	}
}

func TestRouteAndUpstreamGRPC(t *testing.T) {
	connection, backend := newTestGRPCConnection(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	routes := adminv1.NewRouteServiceClient(connection)
	_, err := routes.CreateRoute(ctx, &adminv1.CreateRouteRequest{})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("CreateRoute(empty) code = %s, want InvalidArgument", code)
	}
	if actions := backend.Actions(); len(actions) != 0 {
		t.Errorf("CreateRoute(empty) caused %d API Server calls, want 0", len(actions))
	}

	route, err := routes.CreateRoute(ctx, &adminv1.CreateRouteRequest{
		Name:         "public",
		GatewayName:  "edge",
		ListenerName: "http",
		Path: &adminv1.PathMatch{
			Type:  adminv1.PathMatchType_PATH_MATCH_TYPE_EXACT,
			Value: "/",
		},
		UpstreamName: "backend",
	})
	if err != nil {
		t.Fatalf("CreateRoute(public) error = %v, want nil", err)
	}
	if route.GetRoute().GetName() != "public" {
		t.Errorf("CreateRoute(public) name = %q, want public", route.GetRoute().GetName())
	}

	upstreams := adminv1.NewUpstreamServiceClient(connection)
	upstream, err := upstreams.CreateUpstream(ctx, &adminv1.CreateUpstreamRequest{
		Name: "backend",
		Endpoints: []*adminv1.Endpoint{{
			Address: "127.0.0.1",
			Port:    8080,
			Weight:  1,
		}},
	})
	if err != nil {
		t.Fatalf("CreateUpstream(backend) error = %v, want nil", err)
	}
	if upstream.GetUpstream().GetName() != "backend" {
		t.Errorf("CreateUpstream(backend) name = %q, want backend", upstream.GetUpstream().GetName())
	}
}

func newTestGRPCClient(t *testing.T) (adminv1.GatewayServiceClient, *clientfake.Clientset) {
	t.Helper()
	connection, backend := newTestGRPCConnection(t)
	return adminv1.NewGatewayServiceClient(connection), backend
}

func newTestGRPCConnection(t *testing.T) (*grpc.ClientConn, *clientfake.Clientset) {
	t.Helper()

	_, grpcServer, backend := newTestServers()
	listener := bufconn.Listen(1024 * 1024)
	done := make(chan error, 1)
	go func() {
		done <- grpcServer.Serve(listener)
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := grpcServer.Stop(ctx); err != nil {
			t.Errorf("stop gRPC server: %v", err)
		}
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				t.Errorf("serve gRPC: %v", err)
			}
		case <-ctx.Done():
			t.Error("gRPC server did not stop before deadline")
		}
	})

	connection, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
	)
	if err != nil {
		t.Fatalf("create gRPC client: %v", err)
	}
	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Errorf("close gRPC client: %v", err)
		}
	})

	return connection, backend
}
