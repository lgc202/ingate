package gateway

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestReadResources(t *testing.T) {
	gateway := gatewayResource("edge", 1, listener("http", 8080, ""))
	route := routeResource("route", 2, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	backend := backendResource("backend")
	apiClient := newFakeClient(t, &gateway, &route, &backend)
	r := New(apiClient, nil)

	got, err := r.readResources(t.Context())
	if err != nil {
		t.Fatalf("readResources() error = %v, want nil", err)
	}
	if diff := cmp.Diff([]gatewayv1alpha1.Gateway{gateway}, got.gateways.Items); diff != "" {
		t.Errorf("readResources() Gateways mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]gatewayv1alpha1.HTTPRoute{route}, got.httpRoutes.Items); diff != "" {
		t.Errorf("readResources() HTTPRoutes mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]gatewayv1alpha1.Backend{backend}, got.backends.Items); diff != "" {
		t.Errorf("readResources() Backends mismatch (-want +got):\n%s", diff)
	}
}

func backendResource(name string) gatewayv1alpha1.Backend {
	return gatewayv1alpha1.Backend{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1alpha1.BackendSpec{Endpoints: []gatewayv1alpha1.Endpoint{
			{Address: "127.0.0.1", Port: 8081, Weight: 1},
		}},
	}
}
