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
)

func TestGatewayCRUDHTTP(t *testing.T) {
	seed := &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "edge",
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
		"name": "new-gateway",
		"listeners": [{"name": "http", "protocol": "LISTENER_PROTOCOL_HTTP", "port": 8081}]
	}`
	response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/gateways", created)
	requireHTTPStatus(t, response, http.StatusOK)

	updated := `{
		"version": "stale",
		"listeners": [{"name": "http", "protocol": "LISTENER_PROTOCOL_HTTP", "port": 9090}]
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/gateways/edge", updated)
	requireHTTPStatus(t, response, http.StatusConflict)
	stored, err := client.GatewayV1alpha1().Gateways().Get(context.Background(), "edge", metav1.GetOptions{})
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
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/gateways/edge", updated)
	requireHTTPStatus(t, response, http.StatusOK)
	var updateReply adminv1.UpdateGatewayReply
	decodeReply(t, response, &updateReply)
	if updateReply.GetGateway().GetDisplayName() != "Updated" {
		t.Errorf("UpdateGateway(edge) display name = %q, want Updated", updateReply.GetGateway().GetDisplayName())
	}
	stored, err = client.GatewayV1alpha1().Gateways().Get(context.Background(), "edge", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read updated Gateway: %v", err)
	}
	if stored.Labels["team"] != "core" {
		t.Errorf("UpdateGateway(edge) labels = %v, want team=core preserved", stored.Labels)
	}
	if len(stored.Status.Conditions) != 1 || stored.Status.Conditions[0].Type != "Accepted" {
		t.Errorf("UpdateGateway(edge) status = %v, want Accepted preserved", stored.Status.Conditions)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/gateways/edge", "")
	requireHTTPStatus(t, response, http.StatusOK)
	_, err = client.GatewayV1alpha1().Gateways().Get(context.Background(), "edge", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("Get deleted Gateway(edge) error = %v, want NotFound", err)
	}
}

func TestRouteCRUDHTTP(t *testing.T) {
	seed := &gatewayv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "public",
			ResourceVersion: "7",
			Labels:          map[string]string{"team": "core"},
		},
		Spec: gatewayv1alpha1.RouteSpec{
			GatewayName:  "edge",
			ListenerName: "http",
			Path: gatewayv1alpha1.PathMatch{
				Type:  gatewayv1alpha1.PathMatchPrefix,
				Value: "/old",
			},
			UpstreamName: "backend",
		},
	}
	httpServer, client := crudTestServer(t, seed)

	created := `{
		"name": "new-route",
		"gatewayName": "edge",
		"listenerName": "http",
		"path": {"type": "PATH_MATCH_TYPE_EXACT", "value": "/new"},
		"upstreamName": "backend"
	}`
	response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/routes", created)
	requireHTTPStatus(t, response, http.StatusOK)
	var createReply adminv1.CreateRouteReply
	decodeReply(t, response, &createReply)
	if createReply.GetRoute().GetPath().GetValue() != "/new" {
		t.Errorf("CreateRoute(new-route) path = %q, want /new", createReply.GetRoute().GetPath().GetValue())
	}

	updated := `{
		"version": "stale",
		"gatewayName": "edge",
		"listenerName": "http",
		"path": {"type": "PATH_MATCH_TYPE_EXACT", "value": "/updated"},
		"upstreamName": "backend"
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/routes/public", updated)
	requireHTTPStatus(t, response, http.StatusConflict)
	stored, err := client.GatewayV1alpha1().Routes().Get(context.Background(), "public", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read Route after stale update: %v", err)
	}
	if stored.Spec.Path.Value != "/old" {
		t.Errorf("UpdateRoute(public, stale) path = %q, want unchanged /old", stored.Spec.Path.Value)
	}

	updated = `{
		"version": "7",
		"gatewayName": "edge",
		"listenerName": "http",
		"path": {"type": "PATH_MATCH_TYPE_EXACT", "value": "/updated"},
		"upstreamName": "backend"
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/routes/public", updated)
	requireHTTPStatus(t, response, http.StatusOK)
	var updateReply adminv1.UpdateRouteReply
	decodeReply(t, response, &updateReply)
	if updateReply.GetRoute().GetPath().GetValue() != "/updated" {
		t.Errorf("UpdateRoute(public) path = %q, want /updated", updateReply.GetRoute().GetPath().GetValue())
	}
	stored, err = client.GatewayV1alpha1().Routes().Get(context.Background(), "public", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read updated Route: %v", err)
	}
	if stored.Labels["team"] != "core" {
		t.Errorf("UpdateRoute(public) labels = %v, want team=core preserved", stored.Labels)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/routes/public", "")
	requireHTTPStatus(t, response, http.StatusOK)
	_, err = client.GatewayV1alpha1().Routes().Get(context.Background(), "public", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("Get deleted Route(public) error = %v, want NotFound", err)
	}
}

func TestUpstreamCRUDHTTP(t *testing.T) {
	seed := &gatewayv1alpha1.Upstream{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "backend",
			ResourceVersion: "7",
			Labels:          map[string]string{"team": "core"},
		},
		Spec: gatewayv1alpha1.UpstreamSpec{
			Endpoints: []gatewayv1alpha1.Endpoint{{
				Address: "127.0.0.1",
				Port:    8080,
				Weight:  1,
			}},
		},
	}
	httpServer, client := crudTestServer(t, seed)

	created := `{
		"name": "new-backend",
		"endpoints": [{"address": "127.0.0.1", "port": 8081, "weight": 1}]
	}`
	response := requestResource(httpServer, http.MethodPost, "/api/admin/v1/upstreams", created)
	requireHTTPStatus(t, response, http.StatusOK)
	var createReply adminv1.CreateUpstreamReply
	decodeReply(t, response, &createReply)
	if createReply.GetUpstream().GetEndpoints()[0].GetPort() != 8081 {
		t.Errorf("CreateUpstream(new-backend) port = %d, want 8081", createReply.GetUpstream().GetEndpoints()[0].GetPort())
	}

	updated := `{
		"version": "stale",
		"endpoints": [{"address": "127.0.0.1", "port": 9090, "weight": 1}]
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/upstreams/backend", updated)
	requireHTTPStatus(t, response, http.StatusConflict)
	stored, err := client.GatewayV1alpha1().Upstreams().Get(context.Background(), "backend", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read Upstream after stale update: %v", err)
	}
	if stored.Spec.Endpoints[0].Port != 8080 {
		t.Errorf("UpdateUpstream(backend, stale) port = %d, want unchanged 8080", stored.Spec.Endpoints[0].Port)
	}

	updated = `{
		"version": "7",
		"endpoints": [{"address": "127.0.0.1", "port": 9090, "weight": 1}]
	}`
	response = requestResource(httpServer, http.MethodPut, "/api/admin/v1/upstreams/backend", updated)
	requireHTTPStatus(t, response, http.StatusOK)
	var updateReply adminv1.UpdateUpstreamReply
	decodeReply(t, response, &updateReply)
	if updateReply.GetUpstream().GetEndpoints()[0].GetPort() != 9090 {
		t.Errorf("UpdateUpstream(backend) port = %d, want 9090", updateReply.GetUpstream().GetEndpoints()[0].GetPort())
	}
	stored, err = client.GatewayV1alpha1().Upstreams().Get(context.Background(), "backend", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read updated Upstream: %v", err)
	}
	if stored.Labels["team"] != "core" {
		t.Errorf("UpdateUpstream(backend) labels = %v, want team=core preserved", stored.Labels)
	}

	response = requestResource(httpServer, http.MethodDelete, "/api/admin/v1/upstreams/backend", "")
	requireHTTPStatus(t, response, http.StatusOK)
	_, err = client.GatewayV1alpha1().Upstreams().Get(context.Background(), "backend", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("Get deleted Upstream(backend) error = %v, want NotFound", err)
	}
}

func TestResourceCreateHTTPRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "Route", path: "/api/admin/v1/routes"},
		{name: "Upstream", path: "/api/admin/v1/upstreams"},
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

func crudTestServer(t *testing.T, seed any) (*kratoshttp.Server, *clientfake.Clientset) {
	t.Helper()
	httpServer, _, client := newTestServers()
	ctx := context.Background()

	var err error
	switch resource := seed.(type) {
	case *gatewayv1alpha1.Gateway:
		_, err = client.GatewayV1alpha1().Gateways().Create(ctx, resource, metav1.CreateOptions{})
	case *gatewayv1alpha1.Route:
		_, err = client.GatewayV1alpha1().Routes().Create(ctx, resource, metav1.CreateOptions{})
	case *gatewayv1alpha1.Upstream:
		_, err = client.GatewayV1alpha1().Upstreams().Create(ctx, resource, metav1.CreateOptions{})
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
