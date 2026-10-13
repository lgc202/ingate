package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestStrategyPolicy(t *testing.T) {
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
}

func TestPrepareForCreate(t *testing.T) {
	resource := testGateway()
	resource.Generation = 9
	resource.Status.Conditions = []metav1.Condition{{Type: "Accepted"}}

	NewStrategy(runtime.NewScheme()).PrepareForCreate(context.Background(), resource)

	if resource.Generation != 1 {
		t.Errorf("PrepareForCreate() generation = %d, want 1", resource.Generation)
	}
	if len(resource.Status.Conditions) != 0 {
		t.Errorf("PrepareForCreate() conditions = %v, want none", resource.Status.Conditions)
	}
}

func TestPrepareForUpdate(t *testing.T) {
	tests := []struct {
		name           string
		changeSpec     bool
		wantGeneration int64
	}{
		{name: "metadata only", wantGeneration: 3},
		{name: "spec changed", changeSpec: true, wantGeneration: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := testGateway()
			previous.Generation = 3
			previous.Status.Conditions = []metav1.Condition{{Type: "Accepted"}}

			next := previous.DeepCopy()
			next.Generation = 99
			next.Status = gatewayapi.GatewayStatus{}
			if tt.changeSpec {
				next.Spec.Listeners[0].Port = 8080
			}

			NewStrategy(runtime.NewScheme()).PrepareForUpdate(context.Background(), next, previous)

			if next.Generation != tt.wantGeneration {
				t.Errorf("PrepareForUpdate(changeSpec=%t) generation = %d, want %d",
					tt.changeSpec, next.Generation, tt.wantGeneration)
			}
			if diff := cmp.Diff(previous.Status, next.Status); diff != "" {
				t.Errorf("PrepareForUpdate(changeSpec=%t) status mismatch (-want +got):\n%s",
					tt.changeSpec, diff)
			}
		})
	}
}

func TestStatusStrategy(t *testing.T) {
	previous := testGateway()
	previous.Generation = 3
	previous.ResourceVersion = "1"
	previous.Labels = map[string]string{"environment": "production"}
	previous.Annotations = map[string]string{"owner": "platform"}
	previous.Finalizers = []string{"gateway.ingate.io/cleanup"}

	next := previous.DeepCopy()
	next.ResourceVersion = "2"
	next.Generation = 99
	next.Labels["environment"] = "test"
	next.Annotations["owner"] = "other"
	next.Finalizers = nil
	next.Spec.Listeners[0].Port = 8080
	next.Status.Conditions = []metav1.Condition{{
		Type:               "Accepted",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: 3,
		LastTransitionTime: metav1.NewTime(time.Unix(1, 0)),
		Reason:             "Accepted",
	}}
	wantStatus := *next.Status.DeepCopy()

	strategy := NewStatusStrategy(runtime.NewScheme())
	strategy.PrepareForUpdate(context.Background(), next, previous)

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
	if diff := cmp.Diff(previous.Annotations, next.Annotations); diff != "" {
		t.Errorf("StatusStrategy.PrepareForUpdate() annotations mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(previous.Finalizers, next.Finalizers); diff != "" {
		t.Errorf("StatusStrategy.PrepareForUpdate() finalizers mismatch (-want +got):\n%s", diff)
	}
	if errs := strategy.ValidateUpdate(context.Background(), next, previous); len(errs) != 0 {
		t.Errorf("StatusStrategy.ValidateUpdate() errors = %v, want none", errs)
	}

	next.Status.Conditions[0].ObservedGeneration = 4
	if errs := strategy.ValidateUpdate(context.Background(), next, previous); len(errs) != 1 ||
		errs[0].Field != "status.conditions[0].observedGeneration" {
		t.Errorf("StatusStrategy.ValidateUpdate(observedGeneration=4) errors = %v, want observedGeneration error", errs)
	}
}

func TestResetFields(t *testing.T) {
	version := fieldpath.APIVersion(gatewayv1alpha1.SchemeGroupVersion.String())
	tests := []struct {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) != 1 || tt.got[version] == nil ||
				tt.got[version].Size() != 1 || !tt.got[version].Has(tt.want) {
				t.Errorf("GetResetFields() = %v, want only %q for %q", tt.got, tt.want, version)
			}
		})
	}
}

func testGateway() *gatewayapi.Gateway {
	return &gatewayapi.Gateway{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "edge"},
		Spec: gatewayapi.GatewaySpec{
			Listeners: []gatewayapi.Listener{{Name: "http", Protocol: gatewayapi.ProtocolHTTP, Port: 80}},
		},
	}
}
