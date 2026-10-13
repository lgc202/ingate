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

	if _, err := resources.Gateways("team-a").Create(t.Context(), &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "edge"},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways().Create(edge) error = %v, want nil", err)
	}
	gateway, err := resources.Gateways("team-a").Get(t.Context(), "edge", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Gateways().Get(edge) error = %v, want nil", err)
	}
	if gateway.Name != "edge" {
		t.Errorf("Gateways().Get(edge).Name = %q, want edge", gateway.Name)
	}

	if _, err := resources.HTTPRoutes("team-a").Create(t.Context(), &gatewayv1alpha1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "web"},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("HTTPRoutes().Create(web) error = %v, want nil", err)
	}
	route, err := resources.HTTPRoutes("team-a").Get(t.Context(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("HTTPRoutes().Get(web) error = %v, want nil", err)
	}
	if route.Name != "web" {
		t.Errorf("HTTPRoutes().Get(web).Name = %q, want web", route.Name)
	}

	if _, err := resources.Backends("team-a").Create(t.Context(), &gatewayv1alpha1.Backend{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "backend"},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Backends().Create(backend) error = %v, want nil", err)
	}
	backend, err := resources.Backends("team-a").Get(t.Context(), "backend", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Backends().Get(backend) error = %v, want nil", err)
	}
	if backend.Name != "backend" {
		t.Errorf("Backends().Get(backend).Name = %q, want backend", backend.Name)
	}
}

func TestGeneratedAIBackendClient(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	resources := client.GatewayV1alpha1().AIBackends("team-a")
	backend := &gatewayv1alpha1.AIBackend{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "model-service"},
		Spec: gatewayv1alpha1.AIBackendSpec{
			BackendRef: gatewayv1alpha1.BackendReference{Name: "model-network"},
			Schema:     gatewayv1alpha1.APISchema{Name: gatewayv1alpha1.APISchemaOpenAI, Prefix: "/v1"},
		},
	}
	if _, err := resources.Create(t.Context(), backend, metav1.CreateOptions{}); err != nil {
		t.Fatalf("AIBackends().Create(model-service) error = %v, want nil", err)
	}
	got, err := resources.Get(t.Context(), backend.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("AIBackends().Get(model-service) error = %v, want nil", err)
	}
	if got.Name != backend.Name || got.Spec != backend.Spec {
		t.Errorf("AIBackends().Get(model-service) = %+v, want name %q and spec %+v", got, backend.Name, backend.Spec)
	}
}
