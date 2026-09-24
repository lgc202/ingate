package v1alpha1

import (
	"slices"
	"testing"

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

func TestRouteAndUpstreamConversion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := gateway.AddToScheme(scheme); err != nil {
		t.Fatalf("gateway.AddToScheme() error = %v", err)
	}
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("v1alpha1.AddToScheme() error = %v", err)
	}

	route := &Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route-1"},
		Spec: RouteSpec{
			GatewayName:  "gateway-1",
			ListenerName: "http",
			Path:         PathMatch{Type: PathMatchPrefix, Value: "/api"},
			UpstreamName: "backend-1",
		},
	}
	var internalRoute gateway.Route
	if err := scheme.Convert(route, &internalRoute, nil); err != nil {
		t.Fatalf("scheme.Convert(Route, internal Route) error = %v", err)
	}
	wantRoute := gateway.RouteSpec{
		GatewayName:  "gateway-1",
		ListenerName: "http",
		Path:         gateway.PathMatch{Type: gateway.PathMatchPrefix, Value: "/api"},
		UpstreamName: "backend-1",
	}
	if got := internalRoute.Spec; got != wantRoute {
		t.Errorf("scheme.Convert(Route, internal Route).Spec = %+v, want %+v", got, wantRoute)
	}

	upstream := &Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-1"},
		Spec: UpstreamSpec{
			Endpoints: []Endpoint{{Address: "api.example.com", Port: 8080, Weight: 1}},
		},
	}
	var internalUpstream gateway.Upstream
	if err := scheme.Convert(upstream, &internalUpstream, nil); err != nil {
		t.Fatalf("scheme.Convert(Upstream, internal Upstream) error = %v", err)
	}
	wantEndpoints := []gateway.Endpoint{{Address: "api.example.com", Port: 8080, Weight: 1}}
	if got := internalUpstream.Spec.Endpoints; !slices.Equal(got, wantEndpoints) {
		t.Errorf("scheme.Convert(Upstream, internal Upstream).Spec.Endpoints = %v, want %v", got, wantEndpoints)
	}
}
