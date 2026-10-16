package validation

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateAIBackend(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*gateway.AIBackend)
		wantField string
		wantType  field.ErrorType
	}{
		{name: "valid with unresolved reference", mutate: func(*gateway.AIBackend) {}},
		{
			name:      "empty name",
			mutate:    func(b *gateway.AIBackend) { b.Name = "" },
			wantField: "metadata.name",
			wantType:  field.ErrorTypeRequired,
		},
		{
			name:      "invalid name",
			mutate:    func(b *gateway.AIBackend) { b.Name = "Invalid" },
			wantField: "metadata.name",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "missing namespace",
			mutate:    func(b *gateway.AIBackend) { b.Namespace = "" },
			wantField: "metadata.namespace",
			wantType:  field.ErrorTypeRequired,
		},
		{
			name:      "invalid namespace",
			mutate:    func(b *gateway.AIBackend) { b.Namespace = "Team.A" },
			wantField: "metadata.namespace",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "empty backend",
			mutate:    func(b *gateway.AIBackend) { b.Spec.BackendRef.Name = "" },
			wantField: "spec.backendRef.name",
			wantType:  field.ErrorTypeRequired,
		},
		{
			name:      "invalid backend",
			mutate:    func(b *gateway.AIBackend) { b.Spec.BackendRef.Name = "Invalid" },
			wantField: "spec.backendRef.name",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "empty schema",
			mutate:    func(b *gateway.AIBackend) { b.Spec.Schema.Name = "" },
			wantField: "spec.schema.name",
			wantType:  field.ErrorTypeRequired,
		},
		{
			name:      "unsupported schema",
			mutate:    func(b *gateway.AIBackend) { b.Spec.Schema.Name = "Anthropic" },
			wantField: "spec.schema.name",
			wantType:  field.ErrorTypeNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := validAIBackend()
			tt.mutate(backend)
			errs := ValidateAIBackend(backend)
			if tt.wantField == "" {
				if len(errs) != 0 {
					t.Errorf("ValidateAIBackend(%+v) = %v, want no errors", backend, errs)
				}

				return
			}

			if len(errs) != 1 || errs[0].Field != tt.wantField || errs[0].Type != tt.wantType {
				t.Errorf("ValidateAIBackend(%+v) = %v, want one %s error at %s", backend, errs, tt.wantType, tt.wantField)
			}
		})
	}
}

func TestValidateAIBackendPrefix(t *testing.T) {
	tests := []struct {
		prefix string
		valid  bool
	}{
		{prefix: "", valid: true},
		{prefix: "/", valid: true},
		{prefix: "/v1", valid: true},
		{prefix: "v1beta/openai", valid: true},
		{prefix: "/compatibility/v1/", valid: true},
		{prefix: "https://api.example.com/v1"},
		{prefix: "//api.example.com/v1"},
		{prefix: "//user:password@api.example.com/v1"},
		{prefix: "/v1?version=1"},
		{prefix: "/v1?"},
		{prefix: "/v1#fragment"},
		{prefix: "/v1#"},
		{prefix: "/v1%zz"},
		{prefix: "/v1\n"},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			backend := validAIBackend()
			backend.Spec.Schema.Prefix = tt.prefix
			errs := ValidateAIBackend(backend)
			if tt.valid {
				if len(errs) != 0 {
					t.Errorf("ValidateAIBackend(prefix %q) = %v, want no errors", tt.prefix, errs)
				}

				return
			}

			if len(errs) != 1 || errs[0].Field != "spec.schema.prefix" || errs[0].Type != field.ErrorTypeInvalid {
				t.Errorf("ValidateAIBackend(prefix %q) = %v, want one Invalid error at spec.schema.prefix", tt.prefix, errs)
			}
		})
	}
}

func TestValidateAIBackendUpdate(t *testing.T) {
	previous := validAIBackend()
	previous.ResourceVersion = "1"
	next := previous.DeepCopy()
	next.Spec.DisplayName = "更新的展示名称"
	next.Spec.BackendRef.Name = "another-network"
	if errs := ValidateAIBackendUpdate(next, previous); len(errs) != 0 {
		t.Errorf("ValidateAIBackendUpdate(changed spec) = %v, want no errors", errs)
	}

	next.Name = "renamed"
	errs := ValidateAIBackendUpdate(next, previous)
	if len(errs) != 1 || errs[0].Field != "metadata.name" || errs[0].Type != field.ErrorTypeInvalid {
		t.Errorf("ValidateAIBackendUpdate(renamed) = %v, want one Invalid error at metadata.name", errs)
	}
}

func TestValidateAIBackendStatusUpdate(t *testing.T) {
	previous := validAIBackend()
	previous.ResourceVersion = "1"
	previous.Generation = 3
	next := previous.DeepCopy()
	next.Status.Conditions = []metav1.Condition{{
		Type:               "ResolvedRefs",
		Status:             metav1.ConditionTrue,
		Reason:             "ResolvedRefs",
		Message:            "backend reference resolved",
		ObservedGeneration: 3,
		LastTransitionTime: metav1.NewTime(time.Unix(1, 0)),
	}}
	if errs := ValidateAIBackendStatusUpdate(next, previous); len(errs) != 0 {
		t.Errorf("ValidateAIBackendStatusUpdate(valid) = %v, want no errors", errs)
	}

	next.Status.Conditions[0].ObservedGeneration = 4
	errs := ValidateAIBackendStatusUpdate(next, previous)
	if len(errs) != 1 || errs[0].Field != "status.conditions[0].observedGeneration" || errs[0].Type != field.ErrorTypeInvalid {
		t.Errorf("ValidateAIBackendStatusUpdate(future generation) = %v, want one Invalid observedGeneration error", errs)
	}
}

func validAIBackend() *gateway.AIBackend {
	return &gateway.AIBackend{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "model-service"},
		Spec: gateway.AIBackendSpec{
			DisplayName: "模型服务",
			BackendRef:  gateway.BackendReference{Name: "model-network"},
			Schema:      gateway.APISchema{Name: gateway.APISchemaOpenAI, Prefix: "/v1"},
		},
	}
}
