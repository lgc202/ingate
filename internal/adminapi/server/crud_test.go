package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGatewayCRUDHTTP(t *testing.T) {
	seed := &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "team-a",
			Name:            "edge",
			UID:             types.UID("gateway-instance"),
			ResourceVersion: "7",
			Labels:          map[string]string{"team": "core"},
		},
		Spec: gatewayv1alpha1.GatewaySpec{
			Listeners: []gatewayv1alpha1.Listener{{
				Name:     "http",
				Protocol: gatewayv1alpha1.ProtocolHTTP,
				Port:     8080,
			}},
		},
		Status: gatewayv1alpha1.GatewayStatus{
			Conditions: []metav1.Condition{{
				Type:   "Accepted",
				Status: metav1.ConditionTrue,
			}},
		},
	}
	httpServer, client := crudTestServer(t, seed)

	created := `{
		"displayName": "重复展示名称",
		"listeners": [{"name": "http", "protocol": "LISTENER_PROTOCOL_HTTP", "port": 8081}]
	}`
	response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/namespaces/team-a/gateways", created)
	requireHTTPStatus(t, response, http.StatusOK)

	updated := `{
		"version": "stale",
		"listeners": [{"name": "http", "protocol": "LISTENER_PROTOCOL_HTTP", "port": 9090}]
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/gateways/edge", updated)
	requireHTTPStatus(t, response, http.StatusConflict)
	stored, err := client.GatewayV1alpha1().Gateways("team-a").Get(context.Background(), "edge", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read Gateway after stale update: %v", err)
	}
	if stored.Spec.Listeners[0].Port != 8080 {
		t.Errorf("UpdateGateway(edge, stale) port = %d, want unchanged 8080", stored.Spec.Listeners[0].Port)
	}

	updated = `{
		"version": "7",
		"displayName": "Updated",
		"listeners": [{"name": "http", "protocol": "LISTENER_PROTOCOL_HTTP", "port": 9090}]
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/gateways/edge", updated)
	requireHTTPStatus(t, response, http.StatusOK)
	var updateReply adminv1.UpdateGatewayReply
	decodeReply(t, response, &updateReply)
	if updateReply.GetGateway().GetDisplayName() != "Updated" {
		t.Errorf("UpdateGateway(edge) display name = %q, want Updated", updateReply.GetGateway().GetDisplayName())
	}
	stored, err = client.GatewayV1alpha1().Gateways("team-a").Get(context.Background(), "edge", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read updated Gateway: %v", err)
	}
	if stored.Labels["team"] != "core" {
		t.Errorf("UpdateGateway(edge) labels = %v, want team=core preserved", stored.Labels)
	}
	if len(stored.Status.Conditions) != 1 || stored.Status.Conditions[0].Type != "Accepted" {
		t.Errorf("UpdateGateway(edge) status = %v, want Accepted preserved", stored.Status.Conditions)
	}
	if stored.Name != "edge" || stored.UID != seed.UID {
		t.Errorf("UpdateGateway(rename) name/UID = %q/%q, want edge/%q", stored.Name, stored.UID, seed.UID)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/namespaces/team-a/gateways/edge", "")
	requireHTTPStatus(t, response, http.StatusOK)
	_, err = client.GatewayV1alpha1().Gateways("team-a").Get(context.Background(), "edge", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("Get deleted Gateway(edge) error = %v, want NotFound", err)
	}
}

func TestRemovedResourcePathsAreNotRegistered(t *testing.T) {
	httpServer, client := crudTestServer(t, nil)
	for _, path := range []string{"/api/admin/v1/routes", "/api/admin/v1/upstreams"} {
		response := requestResource(httpServer, http.MethodGet, path, "")
		requireHTTPStatus(t, response, http.StatusNotFound)
	}
	if actions := client.Actions(); len(actions) != 0 {
		t.Errorf("removed paths caused API actions: %v", actions)
	}
}

func TestHTTPRouteCRUDHTTP(t *testing.T) {
	seed := &gatewayv1alpha1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "team-a",
			Name:            "public",
			UID:             types.UID("route-instance"),
			ResourceVersion: "7",
			Labels:          map[string]string{"team": "core"},
		},
		Spec: gatewayv1alpha1.HTTPRouteSpec{
			ParentRefs: []gatewayv1alpha1.ParentReference{{Name: "edge", SectionName: "http"}},
			Rules: []gatewayv1alpha1.HTTPRouteRule{{
				Matches: []gatewayv1alpha1.HTTPRouteMatch{{
					Path: &gatewayv1alpha1.PathMatch{
						Type:  gatewayv1alpha1.PathMatchPrefix,
						Value: "/old",
					},
				}},
				BackendRefs: []gatewayv1alpha1.BackendReference{{Name: "backend"}},
			}},
		},
	}
	httpServer, client := crudTestServer(t, seed)

	created := `{
		"displayName": "重复展示名称",
		"parentRefs": [{"name": "edge", "sectionName": "http"}],
		"rules": [{"matches": [{"path": {"type": "PATH_MATCH_TYPE_EXACT", "value": "/new"}}], "backendRefs": [{"name": "backend"}]}]
	}`
	response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/namespaces/team-a/http-routes", created)
	requireHTTPStatus(t, response, http.StatusOK)
	var createReply adminv1.CreateHTTPRouteReply
	decodeReply(t, response, &createReply)
	response = requestResource(httpServer, http.MethodPost, "/api/admin/v1/namespaces/team-a/http-routes", created)
	requireHTTPStatus(t, response, http.StatusOK)
	var secondReply adminv1.CreateHTTPRouteReply
	decodeReply(t, response, &secondReply)
	if secondReply.GetRoute().GetName() == createReply.GetRoute().GetName() || secondReply.GetRoute().GetDisplayName() != "重复展示名称" {
		t.Errorf("CreateHTTPRoute(duplicate display name) = %v, want distinct ID and unchanged display name", &secondReply)
	}
	if createReply.GetRoute().GetRules()[0].GetMatches()[0].GetPath().GetValue() != "/new" {
		t.Errorf("CreateHTTPRoute(new-route) path = %q, want /new", createReply.GetRoute().GetRules()[0].GetMatches()[0].GetPath().GetValue())
	}

	updated := `{
		"version": "stale",
		"parentRefs": [{"name": "edge", "sectionName": "http"}],
		"rules": [{"matches": [{"path": {"type": "PATH_MATCH_TYPE_EXACT", "value": "/updated"}}], "backendRefs": [{"name": "backend"}]}]
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/http-routes/public", updated)
	requireHTTPStatus(t, response, http.StatusConflict)
	stored, err := client.GatewayV1alpha1().HTTPRoutes("team-a").Get(context.Background(), "public", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read HTTPRoute after stale update: %v", err)
	}
	if stored.Spec.Rules[0].Matches[0].Path.Value != "/old" {
		t.Errorf("UpdateHTTPRoute(public, stale) path = %q, want unchanged /old", stored.Spec.Rules[0].Matches[0].Path.Value)
	}

	updated = `{
		"version": "7",
		"displayName": "改名后的路由",
		"parentRefs": [{"name": "edge", "sectionName": "http"}],
        "rules": [{"matches": [{"path": {"type": "PATH_MATCH_TYPE_EXACT", "value": "/updated"}}], "backendRefs": [{"name": "backend"}]}]
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/http-routes/public", updated)
	requireHTTPStatus(t, response, http.StatusOK)
	var updateReply adminv1.UpdateHTTPRouteReply
	decodeReply(t, response, &updateReply)
	if updateReply.GetRoute().GetRules()[0].GetMatches()[0].GetPath().GetValue() != "/updated" {
		t.Errorf("UpdateHTTPRoute(public) path = %q, want /updated", updateReply.GetRoute().GetRules()[0].GetMatches()[0].GetPath().GetValue())
	}
	stored, err = client.GatewayV1alpha1().HTTPRoutes("team-a").Get(context.Background(), "public", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read updated HTTPRoute: %v", err)
	}
	if stored.Labels["team"] != "core" {
		t.Errorf("UpdateHTTPRoute(public) labels = %v, want team=core preserved", stored.Labels)
	}
	if stored.Name != "public" || stored.UID != seed.UID || stored.Spec.DisplayName != "改名后的路由" {
		t.Errorf("UpdateHTTPRoute(rename) = %v, want same name/UID and updated display name", stored)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/namespaces/team-a/http-routes/public", "")
	requireHTTPStatus(t, response, http.StatusOK)
	_, err = client.GatewayV1alpha1().HTTPRoutes("team-a").Get(context.Background(), "public", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("Get deleted HTTPRoute(public) error = %v, want NotFound", err)
	}
}

func TestHTTPRouteWriteRejectsUnsupportedAndConflictingInput(t *testing.T) {
	for _, tt := range []struct {
		name   string
		body   string
		status int
	}{
		{
			name:   "default match",
			body:   `{"displayName":"订单路由","parentRefs":[{"name":"edge"}],"rules":[{"backendRefs":[{"name":"backend"}]}]}`,
			status: http.StatusOK,
		},
		{
			name:   "unknown match field",
			body:   `{"displayName":"订单路由","parentRefs":[{"name":"edge"}],"rules":[{"matches":[{"method":"POST"}],"backendRefs":[{"name":"backend"}]}]}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "overlapping parent",
			body:   `{"displayName":"订单路由","parentRefs":[{"name":"edge"},{"name":"edge","sectionName":"http"}],"rules":[{"backendRefs":[{"name":"backend"}]}]}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "multiple targets",
			body:   `{"displayName":"订单路由","parentRefs":[{"name":"edge"}],"rules":[{"backendRefs":[{"name":"backend"},{"name":"other"}]}]}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "invalid path",
			body:   `{"displayName":"订单路由","parentRefs":[{"name":"edge"}],"rules":[{"matches":[{"path":{"value":"/api?q=1"}}],"backendRefs":[{"name":"backend"}]}]}`,
			status: http.StatusBadRequest,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			httpServer, client := crudTestServer(t, nil)
			response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/namespaces/team-a/http-routes", tt.body)
			requireHTTPStatus(t, response, tt.status)
			if tt.status != http.StatusOK {
				if actions := client.Actions(); len(actions) != 0 {
					t.Errorf("invalid input caused API actions: %v", actions)
				}
				return
			}
			var reply adminv1.CreateHTTPRouteReply
			decodeReply(t, response, &reply)
			path := reply.GetRoute().GetRules()[0].GetMatches()[0].GetPath()
			if path.GetType() != adminv1.PathMatchType_PATH_MATCH_TYPE_PREFIX || path.GetValue() != "/" {
				t.Errorf("default path = %v", path)
			}
		})
	}
}

func TestBackendCRUDHTTP(t *testing.T) {
	seed := &gatewayv1alpha1.Backend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "team-a",
			Name:            "backend",
			UID:             types.UID("backend-instance"),
			ResourceVersion: "7",
			Labels:          map[string]string{"team": "core"},
		},
		Spec: gatewayv1alpha1.BackendSpec{
			Endpoints: []gatewayv1alpha1.Endpoint{{
				Address: "127.0.0.1",
				Port:    8080,
				Weight:  1,
			}},
		},
	}
	httpServer, client := crudTestServer(t, seed)

	created := `{
		"displayName": "重复展示名称",
		"endpoints": [{"address": "127.0.0.1", "port": 8081, "weight": 1}]
	}`
	response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/namespaces/team-a/backends", created)
	requireHTTPStatus(t, response, http.StatusOK)
	var createReply adminv1.CreateBackendReply
	decodeReply(t, response, &createReply)
	response = requestResource(httpServer, http.MethodPost, "/api/admin/v1/namespaces/team-a/backends", created)
	requireHTTPStatus(t, response, http.StatusOK)
	var secondReply adminv1.CreateBackendReply
	decodeReply(t, response, &secondReply)
	if secondReply.GetBackend().GetName() == createReply.GetBackend().GetName() || secondReply.GetBackend().GetDisplayName() != "重复展示名称" {
		t.Errorf("CreateBackend(duplicate display name) = %v, want distinct ID and unchanged display name", &secondReply)
	}
	if createReply.GetBackend().GetEndpoints()[0].GetPort() != 8081 {
		t.Errorf("CreateBackend(new-backend) port = %d, want 8081", createReply.GetBackend().GetEndpoints()[0].GetPort())
	}

	updated := `{
		"version": "stale",
		"endpoints": [{"address": "127.0.0.1", "port": 9090, "weight": 1}]
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/backends/backend", updated)
	requireHTTPStatus(t, response, http.StatusConflict)
	stored, err := client.GatewayV1alpha1().Backends("team-a").Get(context.Background(), "backend", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read Backend after stale update: %v", err)
	}
	if stored.Spec.Endpoints[0].Port != 8080 {
		t.Errorf("UpdateBackend(backend, stale) port = %d, want unchanged 8080", stored.Spec.Endpoints[0].Port)
	}

	updated = `{
		"version": "7",
		"displayName": "改名后的后端",
		"endpoints": [{"address": "127.0.0.1", "port": 9090, "weight": 1}]
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/namespaces/team-a/backends/backend", updated)
	requireHTTPStatus(t, response, http.StatusOK)
	var updateReply adminv1.UpdateBackendReply
	decodeReply(t, response, &updateReply)
	if updateReply.GetBackend().GetEndpoints()[0].GetPort() != 9090 {
		t.Errorf("UpdateBackend(backend) port = %d, want 9090", updateReply.GetBackend().GetEndpoints()[0].GetPort())
	}
	stored, err = client.GatewayV1alpha1().Backends("team-a").Get(context.Background(), "backend", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read updated Backend: %v", err)
	}
	if stored.Labels["team"] != "core" {
		t.Errorf("UpdateBackend(backend) labels = %v, want team=core preserved", stored.Labels)
	}
	if stored.Name != "backend" || stored.UID != seed.UID || stored.Spec.DisplayName != "改名后的后端" {
		t.Errorf("UpdateBackend(rename) = %v, want same name/UID and updated display name", stored)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/namespaces/team-a/backends/backend", "")
	requireHTTPStatus(t, response, http.StatusOK)
	_, err = client.GatewayV1alpha1().Backends("team-a").Get(context.Background(), "backend", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("Get deleted Backend(backend) error = %v, want NotFound", err)
	}
}

func TestResourceCreateHTTPRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "HTTPRoute", path: "/api/admin/v1/namespaces/team-a/http-routes"},
		{name: "Backend", path: "/api/admin/v1/namespaces/team-a/backends"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpServer, _, client := newTestServers()
			response := requestResource(httpServer, http.MethodPost, tt.path, `{}`)
			requireHTTPStatus(t, response, http.StatusBadRequest)
			if actions := client.Actions(); len(actions) != 0 {
				t.Errorf("POST %s with invalid input caused %d API Server calls, want 0", tt.path, len(actions))
			}
		})
	}
}

func TestHTTPRouteJSONRejectsUnknownFieldsAcrossContentTypes(t *testing.T) {
	for _, contentType := range []string{
		"application/json", "application/protojson; charset=utf-8", "text/json", "application/json; charset=", "",
	} {
		t.Run(contentType, func(t *testing.T) {
			httpServer, client := crudTestServer(t, nil)
			// 使用 protobuf 原字段名，让宽松 encoding/json 也能识别必需字段。
			body := `{"displayName":"订单路由","parent_refs":[{"name":"edge"}],"rules":[{"matches":[{"method":"POST"}],"backend_refs":[{"name":"backend"}]}]}`
			request := httptest.NewRequest(http.MethodPost, "/api/admin/v1/namespaces/team-a/http-routes", strings.NewReader(body))
			request.Header.Set("Content-Type", contentType)
			response := httptest.NewRecorder()
			httpServer.ServeHTTP(response, request)
			requireHTTPStatus(t, response, http.StatusBadRequest)
			if actions := client.Actions(); len(actions) != 0 {
				t.Errorf("unknown match with Content-Type %q caused API actions: %v", contentType, actions)
			}
		})
	}
}

func TestHTTPRouteJSONAcceptsSupportedContentTypes(t *testing.T) {
	for _, contentType := range []string{"application/json", "application/protojson; charset=utf-8", "text/json"} {
		t.Run(contentType, func(t *testing.T) {
			httpServer, _ := crudTestServer(t, nil)
			body := `{"displayName":"订单路由","parent_refs":[{"name":"edge"}],"rules":[{"backend_refs":[{"name":"backend"}]}]}`
			request := httptest.NewRequest(http.MethodPost, "/api/admin/v1/namespaces/team-a/http-routes", strings.NewReader(body))
			request.Header.Set("Content-Type", contentType)
			response := httptest.NewRecorder()
			httpServer.ServeHTTP(response, request)
			requireHTTPStatus(t, response, http.StatusOK)
		})
	}
}

func crudTestServer(t *testing.T, seed any) (*kratoshttp.Server, *clientfake.Clientset) {
	t.Helper()
	httpServer, _, client := newTestServers()
	ctx := context.Background()

	var err error
	switch resource := seed.(type) {
	case nil:
		return httpServer, client
	case *gatewayv1alpha1.Gateway:
		_, err = client.GatewayV1alpha1().Gateways("team-a").Create(ctx, resource, metav1.CreateOptions{})
	case *gatewayv1alpha1.HTTPRoute:
		_, err = client.GatewayV1alpha1().HTTPRoutes("team-a").Create(ctx, resource, metav1.CreateOptions{})
	case *gatewayv1alpha1.Backend:
		_, err = client.GatewayV1alpha1().Backends("team-a").Create(ctx, resource, metav1.CreateOptions{})
	case *gatewayv1alpha1.AIBackend:
		_, err = client.GatewayV1alpha1().AIBackends("team-a").Create(ctx, resource, metav1.CreateOptions{})
	case *gatewayv1alpha1.ModelRoute:
		_, err = client.GatewayV1alpha1().ModelRoutes("team-a").Create(ctx, resource, metav1.CreateOptions{})
	case *gatewayv1alpha1.MCPRoute:
		_, err = client.GatewayV1alpha1().MCPRoutes("team-a").Create(ctx, resource, metav1.CreateOptions{})
	default:
		t.Fatalf("unsupported seed type %T", seed)
	}
	if err != nil {
		t.Fatalf("seed %T: %v", seed, err)
	}

	return httpServer, client
}

func requestResource(httpServer *kratoshttp.Server, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	httpServer.ServeHTTP(response, request)
	return response
}

func requireHTTPStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("HTTP status = %d, want %d; body = %s", response.Code, want, response.Body.String())
	}
}

func decodeReply(t *testing.T, response *httptest.ResponseRecorder, reply proto.Message) {
	t.Helper()
	if err := protojson.Unmarshal(response.Body.Bytes(), reply); err != nil {
		t.Fatalf("decode %T: %v; body = %s", reply, err, response.Body.String())
	}
}
