package namespace

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

func TestStrategyPolicy(t *testing.T) {
	strategy := NewStrategy(runtime.NewScheme())
	if strategy.NamespaceScoped() {
		t.Error("NamespaceScoped() = true, want false")
	}
	if strategy.AllowCreateOnUpdate(context.Background()) {
		t.Error("AllowCreateOnUpdate() = true, want false")
	}
	if !strategy.AllowUnconditionalUpdate(context.Background()) {
		t.Error("AllowUnconditionalUpdate() = false, want standard Namespace true")
	}
}

func TestPrepareForCreate(t *testing.T) {
	tests := []struct {
		name       string
		finalizers []corev1.FinalizerName
		want       []corev1.FinalizerName
	}{
		{name: "empty", want: []corev1.FinalizerName{corev1.FinalizerKubernetes}},
		{name: "custom", finalizers: []corev1.FinalizerName{"example.com/cleanup"}, want: []corev1.FinalizerName{"example.com/cleanup", corev1.FinalizerKubernetes}},
		{name: "existing", finalizers: []corev1.FinalizerName{corev1.FinalizerKubernetes}, want: []corev1.FinalizerName{corev1.FinalizerKubernetes}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := testNamespace()
			namespace.Generation = 7
			namespace.Spec.Finalizers = tt.finalizers
			namespace.Status = corev1.NamespaceStatus{
				Phase:      corev1.NamespaceTerminating,
				Conditions: []corev1.NamespaceCondition{{Type: corev1.NamespaceDeletionDiscoveryFailure}},
			}
			strategy := NewStrategy(runtime.NewScheme())
			strategy.PrepareForCreate(context.Background(), namespace)
			if diff := cmp.Diff(tt.want, namespace.Spec.Finalizers); diff != "" {
				t.Errorf("PrepareForCreate(%s) finalizers mismatch (-want +got):\n%s", tt.name, diff)
			}
			if want := (corev1.NamespaceStatus{Phase: corev1.NamespaceActive}); !cmp.Equal(want, namespace.Status) {
				t.Errorf("PrepareForCreate(%s) status = %+v, want %+v", tt.name, namespace.Status, want)
			}
			if namespace.Generation != 7 {
				t.Errorf("PrepareForCreate(%s) generation = %d, want unchanged 7", tt.name, namespace.Generation)
			}
			if errs := strategy.Validate(context.Background(), namespace); len(errs) != 0 {
				t.Errorf("Validate(%s) errors = %v, want none", tt.name, errs)
			}
		})
	}
}

func TestCanonicalize(t *testing.T) {
	namespace := testNamespace()
	namespace.Labels = map[string]string{corev1.LabelMetadataName: "wrong", "owner": "platform"}
	namespace.GenerateName = "team-"
	NewStrategy(runtime.NewScheme()).Canonicalize(namespace)
	want := map[string]string{corev1.LabelMetadataName: namespace.Name, "owner": "platform"}
	if diff := cmp.Diff(want, namespace.Labels); diff != "" {
		t.Errorf("Canonicalize() labels mismatch (-want +got):\n%s", diff)
	}
	namespace.Labels = nil
	NewStrategy(runtime.NewScheme()).Canonicalize(namespace)
	if got := namespace.Labels[corev1.LabelMetadataName]; got != namespace.Name {
		t.Errorf("Canonicalize(nil labels) metadata.name label = %q, want %q", got, namespace.Name)
	}
}

func TestPrepareForUpdate(t *testing.T) {
	previous := testNamespace()
	before := previous.DeepCopy()
	next := previous.DeepCopy()
	next.Spec.Finalizers = nil
	next.Status.Phase = corev1.NamespaceTerminating
	next.Labels = map[string]string{"owner": "other"}

	strategy := NewStrategy(runtime.NewScheme())
	strategy.PrepareForUpdate(context.Background(), next, previous)
	if diff := cmp.Diff(previous.Spec.Finalizers, next.Spec.Finalizers); diff != "" {
		t.Errorf("PrepareForUpdate() finalizers mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(previous.Status, next.Status); diff != "" {
		t.Errorf("PrepareForUpdate() status mismatch (-want +got):\n%s", diff)
	}
	if got := next.Labels["owner"]; got != "other" {
		t.Errorf("PrepareForUpdate() owner label = %q, want other", got)
	}
	if errs := strategy.ValidateUpdate(context.Background(), next, previous); len(errs) != 0 {
		t.Errorf("ValidateUpdate() errors = %v, want none", errs)
	}
	if diff := cmp.Diff(before, previous); diff != "" {
		t.Errorf("PrepareForUpdate() old object changed (-want +got):\n%s", diff)
	}
}

func TestStatusStrategy(t *testing.T) {
	previous := testNamespace()
	next := previous.DeepCopy()
	next.Spec.Finalizers = nil
	next.Labels = map[string]string{"owner": "other"}
	next.Status.Conditions = []corev1.NamespaceCondition{{
		Type: corev1.NamespaceDeletionDiscoveryFailure, Status: corev1.ConditionFalse,
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
	if got := next.Labels["owner"]; got != "other" {
		t.Errorf("StatusStrategy.PrepareForUpdate() owner label = %q, want standard Namespace metadata update", got)
	}
	if errs := strategy.ValidateUpdate(context.Background(), next, previous); len(errs) != 0 {
		t.Errorf("StatusStrategy.ValidateUpdate() errors = %v, want none", errs)
	}
	next.Status.Phase = corev1.NamespaceTerminating
	if errs := strategy.ValidateUpdate(context.Background(), next, previous); len(errs) != 1 || errs[0].Field != "status.phase" {
		t.Errorf("StatusStrategy.ValidateUpdate(premature terminating) errors = %v, want status.phase error", errs)
	}
}

func TestFinalizeStrategy(t *testing.T) {
	previous := testNamespace()
	next := previous.DeepCopy()
	next.Spec.Finalizers = nil
	next.Status.Phase = corev1.NamespaceTerminating

	strategy := NewFinalizeStrategy(runtime.NewScheme())
	strategy.PrepareForUpdate(context.Background(), next, previous)
	if diff := cmp.Diff(previous.Status, next.Status); diff != "" {
		t.Errorf("FinalizeStrategy.PrepareForUpdate() status mismatch (-want +got):\n%s", diff)
	}
	if len(next.Spec.Finalizers) != 0 {
		t.Errorf("FinalizeStrategy.PrepareForUpdate() finalizers = %v, want none", next.Spec.Finalizers)
	}
	if errs := strategy.ValidateUpdate(context.Background(), next, previous); len(errs) != 0 {
		t.Errorf("FinalizeStrategy.ValidateUpdate() errors = %v, want none", errs)
	}
	next.Spec.Finalizers = []corev1.FinalizerName{"cleanup"}
	if errs := strategy.ValidateUpdate(context.Background(), next, previous); len(errs) != 1 || errs[0].Field != "spec.finalizers[0]" {
		t.Errorf("FinalizeStrategy.ValidateUpdate(bare finalizer) errors = %v, want spec.finalizers[0] error", errs)
	}
}

func TestResetFields(t *testing.T) {
	tests := []struct {
		name string
		got  map[fieldpath.APIVersion]*fieldpath.Set
		want fieldpath.Path
	}{
		{name: "main", got: NewStrategy(runtime.NewScheme()).GetResetFields(), want: fieldpath.MakePathOrDie("status")},
		{name: "status", got: NewStatusStrategy(runtime.NewScheme()).GetResetFields(), want: fieldpath.MakePathOrDie("spec")},
		{name: "finalize", got: NewFinalizeStrategy(runtime.NewScheme()).GetResetFields(), want: fieldpath.MakePathOrDie("status")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) != 1 || tt.got["v1"] == nil || tt.got["v1"].Size() != 1 || !tt.got["v1"].Has(tt.want) {
				t.Errorf("GetResetFields(%s) = %v, want only %v for v1", tt.name, tt.got, tt.want)
			}
		})
	}
}

func testNamespace() *coreapi.Namespace {
	return &coreapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a", UID: "uid", ResourceVersion: "1"},
		Spec:       corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{corev1.FinalizerKubernetes}},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
}
