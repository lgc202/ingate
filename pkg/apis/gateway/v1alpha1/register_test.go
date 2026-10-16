package v1alpha1

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestGatewayConversion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := gateway.AddToScheme(scheme); err != nil {
		t.Fatalf("gateway.AddToScheme() error = %v", err)
	}

	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("v1alpha1.AddToScheme() error = %v", err)
	}

	input := &Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"},
		Spec: GatewaySpec{
			DisplayName: "生产网关",
			Listeners:   []Listener{{Name: "http", Protocol: ProtocolHTTP, Port: 8080}},
		},
	}
	var internal gateway.Gateway
	if err := scheme.Convert(input, &internal, nil); err != nil {
		t.Fatalf("scheme.Convert(Gateway, internal Gateway) error = %v", err)
	}

	if got, want := internal.Name, input.Name; got != want {
		t.Errorf("scheme.Convert(Gateway, internal Gateway).Name = %q, want %q", got, want)
	}

	if got, want := internal.Spec.DisplayName, input.Spec.DisplayName; got != want {
		t.Errorf("scheme.Convert(Gateway, internal Gateway).Spec.DisplayName = %q, want %q", got, want)
	}

	wantListeners := []gateway.Listener{{Name: "http", Protocol: gateway.ProtocolHTTP, Port: 8080}}
	if got := internal.Spec.Listeners; !slices.Equal(got, wantListeners) {
		t.Errorf("scheme.Convert(Gateway, internal Gateway).Spec.Listeners = %v, want %v", got, wantListeners)
	}
}

func TestHTTPRouteAndBackendConversion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := gateway.AddToScheme(scheme); err != nil {
		t.Fatalf("gateway.AddToScheme() error = %v", err)
	}

	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("v1alpha1.AddToScheme() error = %v", err)
	}

	route := &HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "route-1"},
		Spec: HTTPRouteSpec{
			DisplayName: "订单路由",
			ParentRefs:  []ParentReference{{Name: "gateway-1", SectionName: "http"}},
			Rules: []HTTPRouteRule{{
				Matches:     []HTTPRouteMatch{{Path: &PathMatch{Type: PathMatchPrefix, Value: "/api"}}},
				BackendRefs: []BackendReference{{Name: "backend-1"}},
			}},
		},
	}
	var internalHTTPRoute gateway.HTTPRoute
	if err := scheme.Convert(route, &internalHTTPRoute, nil); err != nil {
		t.Fatalf("scheme.Convert(HTTPRoute, internal HTTPRoute) error = %v", err)
	}

	wantHTTPRoute := gateway.HTTPRouteSpec{
		DisplayName: "订单路由",
		ParentRefs:  []gateway.ParentReference{{Name: "gateway-1", SectionName: "http"}},
		Rules: []gateway.HTTPRouteRule{{
			Matches:     []gateway.HTTPRouteMatch{{Path: &gateway.PathMatch{Type: gateway.PathMatchPrefix, Value: "/api"}}},
			BackendRefs: []gateway.BackendReference{{Name: "backend-1"}},
		}},
	}
	if diff := cmp.Diff(wantHTTPRoute, internalHTTPRoute.Spec); diff != "" {
		t.Errorf("HTTPRoute conversion (-want +got):\n%s", diff)
	}

	backend := &Backend{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-1"},
		Spec: BackendSpec{
			DisplayName: "订单后端",
			Endpoints:   []Endpoint{{Address: "api.example.com", Port: 8080, Weight: 1}},
		},
	}
	var internalBackend gateway.Backend
	if err := scheme.Convert(backend, &internalBackend, nil); err != nil {
		t.Fatalf("scheme.Convert(Backend, internal Backend) error = %v", err)
	}

	wantEndpoints := []gateway.Endpoint{{Address: "api.example.com", Port: 8080, Weight: 1}}
	if internalBackend.Spec.DisplayName != "订单后端" {
		t.Errorf("scheme.Convert(Backend).DisplayName = %q, want 订单后端", internalBackend.Spec.DisplayName)
	}

	if got := internalBackend.Spec.Endpoints; !slices.Equal(got, wantEndpoints) {
		t.Errorf("scheme.Convert(Backend, internal Backend).Spec.Endpoints = %v, want %v", got, wantEndpoints)
	}
}
