package route

import (
	"context"
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
	ctx := context.Background()

	if strategy.NamespaceScoped() {
		t.Error("NamespaceScoped() = true, want false")
	}
	if strategy.AllowCreateOnUpdate(ctx) {
		t.Error("AllowCreateOnUpdate() = true, want false")
	}
	if strategy.AllowUnconditionalUpdate(ctx) {
		t.Error("AllowUnconditionalUpdate() = true, want false")
	}

	route := testRoute()
	route.Generation = 9
	route.Status.Conditions = []metav1.Condition{{Type: "Accepted"}}
	strategy.PrepareForCreate(ctx, route)
	if route.Generation != 1 || len(route.Status.Conditions) != 0 {
		t.Errorf("PrepareForCreate(Route) = generation %d, status %#v; want 1 and empty status",
			route.Generation, route.Status)
	}

	previous := testRoute()
	previous.Generation = 3
	previous.Status.Conditions = []metav1.Condition{{Type: "Accepted"}}

	metadataOnly := previous.DeepCopy()
	metadataOnly.Status = gatewayapi.RouteStatus{}
	strategy.PrepareForUpdate(ctx, metadataOnly, previous)
	if metadataOnly.Generation != 3 {
		t.Errorf("PrepareForUpdate(metadata only) generation = %d, want 3", metadataOnly.Generation)
	}

	next := previous.DeepCopy()
	next.Spec.Path.Value = "/v1"
	next.Status = gatewayapi.RouteStatus{}
	strategy.PrepareForUpdate(ctx, next, previous)
	if next.Generation != 4 {
		t.Errorf("PrepareForUpdate(spec changed) generation = %d, want 4", next.Generation)
	}
	if diff := cmp.Diff(previous.Status, next.Status); diff != "" {
		t.Errorf("PrepareForUpdate(spec changed) status mismatch (-want +got):\n%s", diff)
	}
}

func TestStatusStrategy(t *testing.T) {
	previous := testRoute()
	previous.Generation = 3

	next := previous.DeepCopy()
	next.Generation = 99
	next.Spec.Path.Value = "/v1"
	next.Status.Conditions = []metav1.Condition{{Type: "Accepted"}}
	wantStatus := *next.Status.DeepCopy()

	NewStatusStrategy(runtime.NewScheme()).PrepareForUpdate(context.Background(), next, previous)
	if diff := cmp.Diff(previous.Spec, next.Spec); diff != "" {
		t.Errorf("StatusStrategy.PrepareForUpdate() spec mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantStatus, next.Status); diff != "" {
		t.Errorf("StatusStrategy.PrepareForUpdate() status mismatch (-want +got):\n%s", diff)
	}
	if next.Generation != previous.Generation {
		t.Errorf("StatusStrategy.PrepareForUpdate() generation = %d, want %d",
			next.Generation, previous.Generation)
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
				t.Errorf("GetResetFields() = %v, want only %q for %q", tt.got, tt.want, version)
			}
		})
	}
}

func testRoute() *gatewayapi.Route {
	return &gatewayapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "api"},
		Spec: gatewayapi.RouteSpec{
			GatewayName: "edge", ListenerName: "http", UpstreamName: "backend",
			Path: gatewayapi.PathMatch{Type: gatewayapi.PathMatchPrefix, Value: "/api"},
		},
	}
}
