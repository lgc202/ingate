package server_test

import (
	"context"
	"errors"
	"net"
	"strings"
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

func TestCreateGatewayGRPC(t *testing.T) {
	client, backend := newTestGRPCClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.CreateGateway(ctx, &adminv1.CreateGatewayRequest{Namespace: "team-a"})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("CreateGateway(empty) code = %s, want InvalidArgument; error = %v", code, err)
	}
	if actions := backend.Actions(); len(actions) != 0 {
		t.Errorf("CreateGateway(empty) caused %d API Server calls, want 0", len(actions))
	}

	request := &adminv1.CreateGatewayRequest{
		Namespace:   "team-a",
		DisplayName: "订单网关",
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
	if name := reply.GetGateway().GetName(); !strings.HasPrefix(name, "gateway-") {
		t.Errorf("created Gateway name = %q, want generated gateway ID", name)
	}

	second, err := client.CreateGateway(ctx, request)
	if err != nil {
		t.Fatalf("CreateGateway(duplicate display name) error = %v, want nil", err)
	}
	if second.GetGateway().GetName() == reply.GetGateway().GetName() || second.GetGateway().GetDisplayName() != "订单网关" {
		t.Errorf("CreateGateway(duplicate display name) = %v, want same display name and distinct ID", second)
	}

	// 资源标识冲突仍保持公开错误语义，但展示名称重复不产生冲突。
	backend.PrependReactor("create", "gateways", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{Group: "gateway.ingate.io", Resource: "gateways"}, "generated-id")
	})
	_, err = client.CreateGateway(ctx, request)
	if status.Code(err) != codes.Aborted || kratoserrors.Reason(err) != "GATEWAY_ALREADY_EXISTS" {
		t.Errorf("CreateGateway(identity conflict) error = %v, want Aborted/GATEWAY_ALREADY_EXISTS", err)
	}

	backend.PrependReactor("create", "gateways", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("backend unavailable")
	})
	request.DisplayName = "另一个网关"
	_, err = client.CreateGateway(ctx, request)
	if code := status.Code(err); code != codes.Internal {
		t.Errorf("CreateGateway(backend failure) code = %s, want Internal; error = %v", code, err)
	}
	if message := status.Convert(err).Message(); message != "服务器内部错误" {
		t.Errorf("CreateGateway(backend failure) message = %q, want Chinese public error", message)
	}
}

func TestHTTPRouteAndBackendGRPC(t *testing.T) {
	connection, backend := newTestGRPCConnection(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	routes := adminv1.NewHTTPRouteServiceClient(connection)
	_, err := routes.CreateHTTPRoute(ctx, &adminv1.CreateHTTPRouteRequest{Namespace: "team-a"})
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("CreateHTTPRoute(empty) code = %s, want InvalidArgument", code)
	}
	if actions := backend.Actions(); len(actions) != 0 {
		t.Errorf("CreateHTTPRoute(empty) caused %d API Server calls, want 0", len(actions))
	}

	route, err := routes.CreateHTTPRoute(ctx, &adminv1.CreateHTTPRouteRequest{
		Namespace:   "team-a",
		DisplayName: "订单路由",
		ParentRefs:  []*adminv1.ParentReference{{Name: "edge", SectionName: "http"}},
		Rules: []*adminv1.HTTPRouteRule{{
			Matches: []*adminv1.HTTPRouteMatch{{
				Path: &adminv1.PathMatch{Type: adminv1.PathMatchType_PATH_MATCH_TYPE_EXACT, Value: "/"},
			}},
			BackendRefs: []*adminv1.BackendReference{{Name: "backend"}},
		}},
	})
	if err != nil {
		t.Fatalf("CreateHTTPRoute(public) error = %v, want nil", err)
	}
	if !strings.HasPrefix(route.GetRoute().GetName(), "httproute-") || route.GetRoute().GetDisplayName() != "订单路由" {
		t.Errorf("CreateHTTPRoute() = %v, want generated name and 订单路由 display name", route.GetRoute())
	}

	backends := adminv1.NewBackendServiceClient(connection)
	createdBackend, err := backends.CreateBackend(ctx, &adminv1.CreateBackendRequest{
		Namespace:   "team-a",
		DisplayName: "订单后端",
		Endpoints: []*adminv1.Endpoint{{
			Address: "127.0.0.1",
			Port:    8080,
			Weight:  1,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBackend(backend) error = %v, want nil", err)
	}
	if !strings.HasPrefix(createdBackend.GetBackend().GetName(), "backend-") || createdBackend.GetBackend().GetDisplayName() != "订单后端" {
		t.Errorf("CreateBackend(backend) name = %q, want backend", createdBackend.GetBackend().GetName())
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
