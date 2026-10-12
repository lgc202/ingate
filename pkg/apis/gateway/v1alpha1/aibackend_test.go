package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestAIBackendDefaulting(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v, want nil", err)
	}

	for _, tt := range []struct {
		name   APISchemaName
		prefix string
		want   string
	}{
		{name: APISchemaOpenAI, want: "/v1"},
		{name: APISchemaOpenAI, prefix: "/", want: "/"},
		{name: APISchemaOpenAI, prefix: "v1beta/openai", want: "v1beta/openai"},
		{name: "Anthropic"},
		{},
	} {
		backend := AIBackend{Spec: AIBackendSpec{Schema: APISchema{Name: tt.name, Prefix: tt.prefix}}}
		scheme.Default(&backend)
		if got := backend.Spec.Schema.Prefix; got != tt.want {
			t.Errorf("Scheme.Default(AIBackend schema %q prefix %q).Prefix = %q, want %q", tt.name, tt.prefix, got, tt.want)
		}
		if got := backend.Spec.Schema.Name; got != tt.name {
			t.Errorf("Scheme.Default(AIBackend schema %q).Name = %q, want unchanged", tt.name, got)
		}
	}

	list := AIBackendList{Items: []AIBackend{{Spec: AIBackendSpec{Schema: APISchema{Name: APISchemaOpenAI}}}}}
	scheme.Default(&list)
	if got := list.Items[0].Spec.Schema.Prefix; got != "/v1" {
		t.Errorf("Scheme.Default(AIBackendList).Items[0].Prefix = %q, want /v1", got)
	}
}

func TestAIBackendConversion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := gateway.AddToScheme(scheme); err != nil {
		t.Fatalf("gateway.AddToScheme() error = %v, want nil", err)
	}
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v, want nil", err)
	}

	input := &AIBackend{
		ObjectMeta: metav1.ObjectMeta{Name: "model-service", ResourceVersion: "12", Generation: 3},
		Spec: AIBackendSpec{
			DisplayName: "模型服务",
			BackendRef:  BackendReference{Name: "model-network"},
			Schema:      APISchema{Name: APISchemaOpenAI, Prefix: "compatibility/v1"},
		},
		Status: AIBackendStatus{Conditions: []metav1.Condition{{Type: "ResolvedRefs", ObservedGeneration: 3}}},
	}
	var internal gateway.AIBackend
	if err := scheme.Convert(input, &internal, nil); err != nil {
		t.Fatalf("Scheme.Convert(AIBackend, internal) error = %v, want nil", err)
	}
	wantSpec := gateway.AIBackendSpec{
		DisplayName: "模型服务",
		BackendRef:  gateway.BackendReference{Name: "model-network"},
		Schema:      gateway.APISchema{Name: gateway.APISchemaOpenAI, Prefix: "compatibility/v1"},
	}
	if diff := cmp.Diff(wantSpec, internal.Spec); diff != "" {
		t.Errorf("Scheme.Convert(AIBackend, internal).Spec mismatch (-want +got):\n%s", diff)
	}

	var roundTrip AIBackend
	if err := scheme.Convert(&internal, &roundTrip, nil); err != nil {
		t.Fatalf("Scheme.Convert(internal AIBackend, external) error = %v, want nil", err)
	}
	if diff := cmp.Diff(input, &roundTrip); diff != "" {
		t.Errorf("AIBackend conversion round trip mismatch (-want +got):\n%s", diff)
	}
}
