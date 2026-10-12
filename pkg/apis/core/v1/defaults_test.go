package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNamespaceDefaults(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	tests := []struct {
		name       string
		namespace  corev1.Namespace
		wantLabels map[string]string
		wantPhase  corev1.NamespacePhase
	}{
		{
			name:       "named",
			namespace:  corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}},
			wantLabels: map[string]string{corev1.LabelMetadataName: "team-a"},
			wantPhase:  corev1.NamespaceActive,
		},
		{
			name: "explicit_values",
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{
					"owner": "platform", corev1.LabelMetadataName: "wrong",
				}},
				Status: corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating},
			},
			wantLabels: map[string]string{"owner": "platform", corev1.LabelMetadataName: "team-a"},
			wantPhase:  corev1.NamespaceTerminating,
		},
		{
			name:      "generated_name_pending",
			namespace: corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "team-"}},
			wantPhase: corev1.NamespaceActive,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := tt.namespace.DeepCopy()
			scheme.Default(namespace)
			if diff := cmp.Diff(tt.wantLabels, namespace.Labels); diff != "" {
				t.Errorf("default labels mismatch (-want +got):\n%s", diff)
			}
			if namespace.Status.Phase != tt.wantPhase {
				t.Errorf("default phase = %q, want %q", namespace.Status.Phase, tt.wantPhase)
			}
			if len(namespace.Spec.Finalizers) != 0 {
				t.Errorf("default spec.finalizers = %v, want empty", namespace.Spec.Finalizers)
			}
			once := namespace.DeepCopy()
			scheme.Default(namespace)
			if diff := cmp.Diff(once, namespace); diff != "" {
				t.Errorf("defaults are not idempotent (-want +got):\n%s", diff)
			}

			list := &corev1.NamespaceList{Items: []corev1.Namespace{*tt.namespace.DeepCopy()}}
			scheme.Default(list)
			if diff := cmp.Diff(once, &list.Items[0]); diff != "" {
				t.Errorf("list item defaults mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNamespaceFieldLabels(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	kind := corev1.SchemeGroupVersion.WithKind("Namespace")
	for _, label := range []string{"metadata.name", "status.phase"} {
		gotLabel, gotValue, err := scheme.ConvertFieldLabel(kind, label, "value")
		if err != nil {
			t.Fatalf("ConvertFieldLabel(%q) error = %v", label, err)
		}
		if gotLabel != label || gotValue != "value" {
			t.Errorf("ConvertFieldLabel(%q) = (%q, %q), want (%q, value)", label, gotLabel, gotValue, label)
		}
	}
	for _, label := range []string{"name", "metadata.namespace", "spec.finalizers"} {
		if _, _, err := scheme.ConvertFieldLabel(kind, label, "value"); err == nil {
			t.Errorf("ConvertFieldLabel(%q) error = nil, want unsupported field", label)
		}
	}
}
