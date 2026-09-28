package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	accounthandler "github.com/lgc202/ingate/internal/adminapi/handler/account"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	routehandler "github.com/lgc202/ingate/internal/adminapi/handler/route"
	upstreamhandler "github.com/lgc202/ingate/internal/adminapi/handler/upstream"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	mysqlstore "github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	"github.com/lgc202/ingate/internal/adminapi/server"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

const createGatewayBody = `{"name":"public-gateway","listeners":[{"name":"http","protocol":"LISTENER_PROTOCOL_HTTP","port":8080}]}`

func TestCreateGatewayHTTP(t *testing.T) {
	httpServer, _, _ := newTestServers()

	response := postGateway(httpServer, createGatewayBody, "application/json")
	if response.Code != http.StatusOK {
		t.Fatalf("POST /api/admin/v1/gateways status = %d, want 200; body = %s", response.Code, response.Body.String())
	}

	var reply adminv1.CreateGatewayReply
	if err := protojson.Unmarshal(response.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode CreateGatewayReply: %v; body = %s", err, response.Body.String())
	}
	if name := reply.GetGateway().GetName(); name != "public-gateway" {
		t.Errorf("created Gateway name = %q, want public-gateway", name)
	}

	response = postGateway(httpServer, createGatewayBody, "application/json")
	if response.Code != http.StatusConflict {
		t.Errorf("POST duplicate Gateway status = %d, want 409; body = %s", response.Code, response.Body.String())
	}
}

func TestCreateGatewayHTTPProtoJSON(t *testing.T) {
	httpServer, _, _ := newTestServers()

	response := postGateway(httpServer, createGatewayBody, "application/protojson")
	if response.Code != http.StatusOK {
		t.Fatalf("POST /api/admin/v1/gateways with ProtoJSON status = %d, want 200; body = %s", response.Code, response.Body.String())
	}

	var reply adminv1.CreateGatewayReply
	if err := protojson.Unmarshal(response.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode ProtoJSON CreateGatewayReply: %v", err)
	}
	if name := reply.GetGateway().GetName(); name != "public-gateway" {
		t.Errorf("created Gateway name = %q, want public-gateway", name)
	}

	response = postGateway(httpServer, createGatewayBody, "application/protojson")
	if response.Code != http.StatusConflict {
		t.Errorf("POST duplicate Gateway with ProtoJSON status = %d, want 409; body = %s", response.Code, response.Body.String())
	}
}

func TestCreateGatewayHTTPRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid contract", body: `{}`},
		{name: "malformed JSON", body: `{"name":`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpServer, _, client := newTestServers()
			response := postGateway(httpServer, tt.body, "application/json")
			if response.Code != http.StatusBadRequest {
				t.Errorf("POST invalid Gateway status = %d, want 400; body = %s", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), "请求参数不合法") {
				t.Errorf("POST invalid Gateway body = %s, want Chinese public error", response.Body.String())
			}
			if actions := client.Actions(); len(actions) != 0 {
				t.Errorf("POST invalid Gateway caused %d API Server calls, want 0", len(actions))
			}
		})
	}
}

func TestCreateGatewayHTTPHidesBackendError(t *testing.T) {
	httpServer, _, client := newTestServers()
	client.PrependReactor("create", "gateways", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("backend unavailable")
	})

	response := postGateway(httpServer, createGatewayBody, "application/json")
	if response.Code != http.StatusInternalServerError {
		t.Errorf("POST Gateway with failed backend status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "backend unavailable") {
		t.Errorf("POST Gateway with failed backend leaked backend error: %s", response.Body.String())
	}
}

func TestCreateGatewayHTTPOldPathNotRegistered(t *testing.T) {
	httpServer, _, client := newTestServers()
	request := httptest.NewRequest(http.MethodPost, "/v1/gateways", strings.NewReader(createGatewayBody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	httpServer.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Errorf("POST /v1/gateways status = %d, want 404", response.Code)
	}
	if actions := client.Actions(); len(actions) != 0 {
		t.Errorf("POST /v1/gateways caused %d API Server calls, want 0", len(actions))
	}
}

func TestAdminAPIAuthenticationSelection(t *testing.T) {
	reject := func(next kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(context.Context, any) (any, error) {
			return nil, kratoserrors.Unauthorized("AUTH_SESSION_REQUIRED", "请先登录")
		}
	}
	httpServer, _, client := newTestServersWithAuthentication(reject)

	response := postGateway(httpServer, createGatewayBody, "application/json")
	if response.Code != http.StatusUnauthorized {
		t.Errorf("POST Gateway without Session status = %d, want 401", response.Code)
	}
	if actions := client.Actions(); len(actions) != 0 {
		t.Errorf("POST Gateway without Session caused %d API Server calls, want 0", len(actions))
	}
	for _, path := range []string{"/api/admin/v1/routes", "/api/admin/v1/upstreams"} {
		response = requestResource(httpServer, http.MethodPost, path, `{}`)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("POST %s without Session status = %d, want 401", path, response.Code)
		}
	}
	if actions := client.Actions(); len(actions) != 0 {
		t.Errorf("POST resources without Session caused %d API Server calls, want 0", len(actions))
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/v1/accounts/me:changePassword",
		strings.NewReader(`{"currentPassword":"admin","newPassword":"new-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	httpServer.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Errorf("POST ChangePassword without Session status = %d, want 401", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/admin/v1/auth/login", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	httpServer.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Errorf("POST invalid Login without Session status = %d, want 400", response.Code)
	}
}

func newTestServers() (*kratoshttp.Server, *kratosgrpc.Server, *clientfake.Clientset) {
	allow := func(next kratosmiddleware.Handler) kratosmiddleware.Handler { return next }

	return newTestServersWithAuthentication(allow)
}

func newTestServersWithAuthentication(authentication kratosmiddleware.Middleware) (
	*kratoshttp.Server,
	*kratosgrpc.Server,
	*clientfake.Clientset,
) {
	return newTestServersWithStore(authentication, nil)
}

func newTestServersWithStore(
	authentication kratosmiddleware.Middleware,
	store *mysqlstore.Store,
) (*kratoshttp.Server, *kratosgrpc.Server, *clientfake.Clientset) {
	client := clientfake.NewSimpleClientset()
	gateways := repository.NewGateway(client.GatewayV1alpha1().Gateways(), store)
	routes := repository.NewRoute(client.GatewayV1alpha1().Routes(), store)
	upstreams := repository.NewUpstream(client.GatewayV1alpha1().Upstreams(), store)

	gatewayHandler := gatewayhandler.New(gatewayservice.New(gateways))
	routeHandler := routehandler.New(routeservice.New(routes))
	upstreamHandler := upstreamhandler.New(upstreamservice.New(upstreams))

	config := &conf.Server{
		Http: &conf.Server_HTTP{
			Addr:    "127.0.0.1:0",
			Timeout: durationpb.New(5 * time.Second),
		},
		Grpc: &conf.Server_GRPC{
			Addr:    "127.0.0.1:0",
			Timeout: durationpb.New(5 * time.Second),
		},
	}

	handlers := server.Handlers{
		Gateway:  gatewayHandler,
		Route:    routeHandler,
		Upstream: upstreamHandler,
		Account:  accounthandler.New(nil),
	}
	chain := server.NewMiddlewareChain(authentication)
	httpServer, grpcServer := server.New(config, handlers, chain)

	return httpServer, grpcServer, client
}

func postGateway(httpServer *kratoshttp.Server, body, mediaType string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/admin/v1/gateways", strings.NewReader(body))
	request.Header.Set("Content-Type", mediaType)
	request.Header.Set("Accept", mediaType)

	response := httptest.NewRecorder()
	httpServer.ServeHTTP(response, request)

	return response
}
