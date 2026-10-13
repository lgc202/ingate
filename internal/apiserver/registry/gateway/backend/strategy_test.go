package backend

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

	if !strategy.NamespaceScoped() {
		t.Error("NamespaceScoped() = false, want true")
	}
	if strategy.AllowCreateOnUpdate(ctx) {
		t.Error("AllowCreateOnUpdate() = true, want false")
	}
	if strategy.AllowUnconditionalUpdate(ctx) {
		t.Error("AllowUnconditionalUpdate() = true, want false")
	}

	backend := testBackend()
	backend.Generation = 9
	backend.Status.Conditions = []metav1.Condition{{Type: "Accepted"}}
	strategy.PrepareForCreate(ctx, backend)
	if backend.Generation != 1 || len(backend.Status.Conditions) != 0 {
		t.Errorf("PrepareForCreate(Backend) = generation %d, status %#v; want 1 and empty status",
			backend.Generation, backend.Status)
	}

	previous := testBackend()
	previous.Generation = 3
	previous.Status.Conditions = []metav1.Condition{{Type: "Accepted"}}

	metadataOnly := previous.DeepCopy()
	metadataOnly.Status = gatewayapi.BackendStatus{}
	strategy.PrepareForUpdate(ctx, metadataOnly, previous)
	if metadataOnly.Generation != 3 {
		t.Errorf("PrepareForUpdate(metadata only) generation = %d, want 3", metadataOnly.Generation)
	}

	next := previous.DeepCopy()
	next.Spec.Endpoints[0].Port = 8081
	next.Status = gatewayapi.BackendStatus{}
	strategy.PrepareForUpdate(ctx, next, previous)
	if next.Generation != 4 {
		t.Errorf("PrepareForUpdate(spec changed) generation = %d, want 4", next.Generation)
	}
	if diff := cmp.Diff(previous.Status, next.Status); diff != "" {
		t.Errorf("PrepareForUpdate(spec changed) status mismatch (-want +got):\n%s", diff)
	}
}

func TestStrategyCanonicalizesEndpointOrder(t *testing.T) {
	strategy := NewStrategy(runtime.NewScheme())
	ctx := context.Background()
	first := gatewayapi.Endpoint{Address: "a.example.com", Port: 8080, Weight: 1}
	second := gatewayapi.Endpoint{Address: "b.example.com", Port: 8080, Weight: 2}

	created := testBackend()
	created.Spec.Endpoints = []gatewayapi.Endpoint{second, first}
	strategy.PrepareForCreate(ctx, created)
	if diff := cmp.Diff([]gatewayapi.Endpoint{first, second}, created.Spec.Endpoints); diff != "" {
		t.Errorf("PrepareForCreate() endpoints mismatch (-want +got):\n%s", diff)
	}

	previous := created.DeepCopy()
	previous.Generation = 3
	next := previous.DeepCopy()
	next.Spec.Endpoints[0], next.Spec.Endpoints[1] = next.Spec.Endpoints[1], next.Spec.Endpoints[0]
	strategy.PrepareForUpdate(ctx, next, previous)
	if diff := cmp.Diff(previous.Spec.Endpoints, next.Spec.Endpoints); diff != "" {
		t.Errorf("PrepareForUpdate(reordered endpoints) mismatch (-want +got):\n%s", diff)
	}
	if next.Generation != previous.Generation {
		t.Errorf("PrepareForUpdate(reordered endpoints) generation = %d, want %d", next.Generation, previous.Generation)
	}
}

func TestStatusStrategy(t *testing.T) {
	previous := testBackend()
	previous.Generation = 3
	previous.Labels = map[string]string{"environment": "production"}
	previous.Finalizers = []string{"gateway.ingate.io/cleanup"}

	next := previous.DeepCopy()
	next.Generation = 99
	next.Labels["environment"] = "test"
	next.Finalizers = nil
	next.Spec.Endpoints[0].Port = 8081
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
	if diff := cmp.Diff(previous.Labels, next.Labels); diff != "" {
		t.Errorf("StatusStrategy.PrepareForUpdate() labels mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(previous.Finalizers, next.Finalizers); diff != "" {
		t.Errorf("StatusStrategy.PrepareForUpdate() finalizers mismatch (-want +got):\n%s", diff)
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

func testBackend() *gatewayapi.Backend {
	return &gatewayapi.Backend{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "backend"},
		Spec: gatewayapi.BackendSpec{
			Endpoints: []gatewayapi.Endpoint{{Address: "127.0.0.1", Port: 8080, Weight: 1}},
		},
	}
}
