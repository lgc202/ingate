package versioned_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestGeneratedResourceClients(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	resources := client.GatewayV1alpha1()

	if _, err := resources.Gateways().Create(t.Context(), &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "edge"},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways().Create(edge) error = %v, want nil", err)
	}
	gateway, err := resources.Gateways().Get(t.Context(), "edge", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Gateways().Get(edge) error = %v, want nil", err)
	}
	if gateway.Name != "edge" {
		t.Errorf("Gateways().Get(edge).Name = %q, want edge", gateway.Name)
	}

	if _, err := resources.Routes().Create(t.Context(), &gatewayv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "web"},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Routes().Create(web) error = %v, want nil", err)
	}
	route, err := resources.Routes().Get(t.Context(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Routes().Get(web) error = %v, want nil", err)
	}
	if route.Name != "web" {
		t.Errorf("Routes().Get(web).Name = %q, want web", route.Name)
	}

	if _, err := resources.Upstreams().Create(t.Context(), &gatewayv1alpha1.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "backend"},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Upstreams().Create(backend) error = %v, want nil", err)
	}
	upstream, err := resources.Upstreams().Get(t.Context(), "backend", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Upstreams().Get(backend) error = %v, want nil", err)
	}
	if upstream.Name != "backend" {
		t.Errorf("Upstreams().Get(backend).Name = %q, want backend", upstream.Name)
	}
}
