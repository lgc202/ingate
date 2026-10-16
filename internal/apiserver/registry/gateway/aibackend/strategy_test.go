package aibackend

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestStrategy(t *testing.T) {
	strategy := NewStrategy(runtime.NewScheme())
	ctx := t.Context()
	if !strategy.NamespaceScoped() {
		t.Error("NamespaceScoped() = false, want true")
	}

	if strategy.AllowCreateOnUpdate(ctx) {
		t.Error("AllowCreateOnUpdate() = true, want false")
	}

	if strategy.AllowUnconditionalUpdate(ctx) {
		t.Error("AllowUnconditionalUpdate() = true, want false")
	}

	backend := testAIBackend()
	backend.Generation = 9
	backend.Status.Conditions = []metav1.Condition{{Type: "Accepted"}}
	strategy.PrepareForCreate(ctx, backend)
	if backend.Generation != 1 || len(backend.Status.Conditions) != 0 {
		t.Errorf("PrepareForCreate(AIBackend) = generation %d, status %#v; want 1 and empty status", backend.Generation, backend.Status)
	}

	if errs := strategy.Validate(ctx, backend); len(errs) != 0 {
		t.Errorf("Validate(valid AIBackend) = %v, want no errors", errs)
	}

	backend.Spec.Schema.Name = "Anthropic"
	if errs := strategy.Validate(ctx, backend); len(errs) == 0 {
		t.Error("Validate(unsupported schema) = no errors, want validation error")
	}

	previous := testAIBackend()
	previous.ResourceVersion = "1"
	previous.Generation = 3
	previous.Status.Conditions = []metav1.Condition{{Type: "ResolvedRefs"}}
	metadataOnly := previous.DeepCopy()
	metadataOnly.Labels = map[string]string{"environment": "test"}
	metadataOnly.Status = gatewayapi.AIBackendStatus{}
	strategy.PrepareForUpdate(ctx, metadataOnly, previous)
	if metadataOnly.Generation != previous.Generation {
		t.Errorf("PrepareForUpdate(metadata only).Generation = %d, want %d", metadataOnly.Generation, previous.Generation)
	}

	if diff := cmp.Diff(previous.Status, metadataOnly.Status); diff != "" {
		t.Errorf("PrepareForUpdate(metadata only).Status mismatch (-want +got):\n%s", diff)
	}

	next := previous.DeepCopy()
	next.Spec.Schema.Prefix = "compatibility/v1"
	next.Status = gatewayapi.AIBackendStatus{}
	strategy.PrepareForUpdate(ctx, next, previous)
	if next.Generation != previous.Generation+1 {
		t.Errorf("PrepareForUpdate(changed prefix).Generation = %d, want %d", next.Generation, previous.Generation+1)
	}

	if diff := cmp.Diff(previous.Status, next.Status); diff != "" {
		t.Errorf("PrepareForUpdate(changed prefix).Status mismatch (-want +got):\n%s", diff)
	}

	if errs := strategy.ValidateUpdate(ctx, next, previous); len(errs) != 0 {
		t.Errorf("ValidateUpdate(valid AIBackend) = %v, want no errors", errs)
	}

	next.Spec.BackendRef.Name = ""
	if errs := strategy.ValidateUpdate(ctx, next, previous); len(errs) == 0 {
		t.Error("ValidateUpdate(empty backend reference) = no errors, want validation error")
	}
}

func TestStatusStrategy(t *testing.T) {
	previous := testAIBackend()
	previous.Generation = 3
	previous.Labels = map[string]string{"environment": "production"}
	previous.Finalizers = []string{"gateway.ingate.io/cleanup"}

	next := previous.DeepCopy()
	next.Generation = 99
	next.Labels["environment"] = "test"
	next.Finalizers = nil
	next.Spec.BackendRef.Name = "other-network"
	next.Status.Conditions = []metav1.Condition{{Type: "ResolvedRefs"}}
	wantStatus := *next.Status.DeepCopy()

	NewStatusStrategy(runtime.NewScheme()).PrepareForUpdate(t.Context(), next, previous)
	if diff := cmp.Diff(previous.Spec, next.Spec); diff != "" {
		t.Errorf("StatusStrategy.PrepareForUpdate().Spec mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(wantStatus, next.Status); diff != "" {
		t.Errorf("StatusStrategy.PrepareForUpdate().Status mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(previous.ObjectMeta, next.ObjectMeta); diff != "" {
		t.Errorf("StatusStrategy.PrepareForUpdate().ObjectMeta mismatch (-want +got):\n%s", diff)
	}
}

func TestResetFields(t *testing.T) {
	version := fieldpath.APIVersion(gatewayv1alpha1.SchemeGroupVersion.String())
	for _, tt := range []struct {
		name string
		got  map[fieldpath.APIVersion]*fieldpath.Set
		want fieldpath.Path
	}{
		{
			name: "main resource",
			got:  NewStrategy(runtime.NewScheme()).GetResetFields(),
			want: fieldpath.MakePathOrDie("status"),
		},
		{
			name: "status",
			got:  NewStatusStrategy(runtime.NewScheme()).GetResetFields(),
			want: fieldpath.MakePathOrDie("spec"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) != 1 || tt.got[version] == nil || tt.got[version].Size() != 1 || !tt.got[version].Has(tt.want) {
				t.Errorf("GetResetFields(%s) = %v, want only %q for %q", tt.name, tt.got, tt.want, version)
			}
		})
	}
}

func testAIBackend() *gatewayapi.AIBackend {
	return &gatewayapi.AIBackend{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "model-service"},
		Spec: gatewayapi.AIBackendSpec{
			BackendRef: gatewayapi.BackendReference{Name: "model-network"},
			Schema:     gatewayapi.APISchema{Name: gatewayapi.APISchemaOpenAI, Prefix: "/v1"},
		},
	}
}
