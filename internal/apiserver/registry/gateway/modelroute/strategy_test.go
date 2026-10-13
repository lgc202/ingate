package modelroute

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestStrategyProtectsStatusAndGeneration(t *testing.T) {
	strategy := NewStrategy(runtime.NewScheme())
	ctx := t.Context()
	if !strategy.NamespaceScoped() {
		t.Error("NamespaceScoped() = false, want true")
	}
	if strategy.AllowCreateOnUpdate(ctx) || strategy.AllowUnconditionalUpdate(ctx) {
		t.Error("ModelRoute strategy permits create-on-update or unconditional update, want both false")
	}

	route := testModelRoute()
	route.Generation = 9
	route.Status.Parents = []gatewayapi.RouteParentStatus{{ControllerName: "gateway.ingate.io/test"}}
	strategy.PrepareForCreate(ctx, route)
	if route.Generation != 1 || len(route.Status.Parents) != 0 {
		t.Errorf("PrepareForCreate(ModelRoute) = generation %d, status %#v; want 1 and empty status", route.Generation, route.Status)
	}
	if errs := strategy.Validate(ctx, route); len(errs) != 0 {
		t.Errorf("Validate(valid ModelRoute) = %v, want no errors", errs)
	}
	route.Spec.Rules[0].BackendRefs = nil
	if errs := strategy.Validate(ctx, route); len(errs) == 0 {
		t.Error("Validate(no AIBackend) = no errors, want validation error")
	}

	previous := testModelRoute()
	previous.ResourceVersion = "1"
	previous.Generation = 3
	previous.Status.Parents = []gatewayapi.RouteParentStatus{{ControllerName: "gateway.ingate.io/test"}}
	metadataOnly := previous.DeepCopy()
	metadataOnly.Labels = map[string]string{"environment": "test"}
	metadataOnly.Status = gatewayapi.ModelRouteStatus{}
	strategy.PrepareForUpdate(ctx, metadataOnly, previous)
	if metadataOnly.Generation != previous.Generation {
		t.Errorf("PrepareForUpdate(metadata only).Generation = %d, want %d", metadataOnly.Generation, previous.Generation)
	}
	if diff := cmp.Diff(previous.Status, metadataOnly.Status); diff != "" {
		t.Errorf("PrepareForUpdate(metadata only).Status mismatch (-want +got):\n%s", diff)
	}

	next := previous.DeepCopy()
	next.Spec.Rules[0].BackendRefs[0].ModelNameOverride = "code-v2"
	next.Status = gatewayapi.ModelRouteStatus{}
	strategy.PrepareForUpdate(ctx, next, previous)
	if next.Generation != previous.Generation+1 {
		t.Errorf("PrepareForUpdate(changed override).Generation = %d, want %d", next.Generation, previous.Generation+1)
	}
	if diff := cmp.Diff(previous.Status, next.Status); diff != "" {
		t.Errorf("PrepareForUpdate(changed override).Status mismatch (-want +got):\n%s", diff)
	}
	if errs := strategy.ValidateUpdate(ctx, next, previous); len(errs) != 0 {
		t.Errorf("ValidateUpdate(valid ModelRoute) = %v, want no errors", errs)
	}
	next.Spec.Rules[0].BackendRefs[0].Name = ""
	if errs := strategy.ValidateUpdate(ctx, next, previous); len(errs) == 0 {
		t.Error("ValidateUpdate(empty AIBackend reference) = no errors, want validation error")
	}
}

func TestStatusStrategyProtectsSpecAndMetadata(t *testing.T) {
	previous := testModelRoute()
	previous.Generation = 3
	previous.Labels = map[string]string{"environment": "production"}
	previous.Finalizers = []string{"gateway.ingate.io/cleanup"}
	next := previous.DeepCopy()
	next.Generation = 99
	next.Labels["environment"] = "test"
	next.Finalizers = nil
	next.Spec.Rules[0].BackendRefs[0].Name = "other-service"
	next.Status.Parents = []gatewayapi.RouteParentStatus{{ControllerName: "gateway.ingate.io/test"}}
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

func testModelRoute() *gatewayapi.ModelRoute {
	return &gatewayapi.ModelRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "models"},
		Spec: gatewayapi.ModelRouteSpec{
			ParentRefs: []gatewayapi.ParentReference{{Name: "edge"}},
			Rules: []gatewayapi.ModelRouteRule{{
				BackendRefs: []gatewayapi.AIBackendReference{{Name: "model-service"}},
			}},
		},
	}
}
