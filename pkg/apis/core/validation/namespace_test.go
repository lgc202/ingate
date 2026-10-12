package validation

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name       string
		objectName string
		namespace  string
		finalizer  corev1.FinalizerName
		wantField  string
	}{
		{name: "standard", objectName: "team-a", finalizer: corev1.FinalizerKubernetes},
		{name: "qualified", objectName: "team-a", finalizer: "example.com/cleanup"},
		{name: "orphan", objectName: "team-a", finalizer: corev1.FinalizerName(metav1.FinalizerOrphanDependents)},
		{name: "foreground", objectName: "team-a", finalizer: corev1.FinalizerName(metav1.FinalizerDeleteDependents)},
		{name: "dotted name", objectName: "team.a", finalizer: corev1.FinalizerKubernetes, wantField: "metadata.name"},
		{name: "long name", objectName: strings.Repeat("a", 64), finalizer: corev1.FinalizerKubernetes, wantField: "metadata.name"},
		{name: "namespaced Namespace", objectName: "team-a", namespace: "parent", finalizer: corev1.FinalizerKubernetes, wantField: "metadata.namespace"},
		{name: "bare custom finalizer", objectName: "team-a", finalizer: "cleanup", wantField: "spec.finalizers"},
		{name: "malformed finalizer", objectName: "team-a", finalizer: "example.com/bad finalizer", wantField: "spec.finalizers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := &coreapi.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: tt.objectName, Namespace: tt.namespace},
				Spec:       corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{tt.finalizer}},
			}
			errs := ValidateNamespace(namespace)
			if tt.wantField == "" {
				if len(errs) != 0 {
					t.Errorf("ValidateNamespace(%+v) errors = %v, want none", namespace, errs)
				}
				return
			}
			if len(errs) != 1 || errs[0].Field != tt.wantField {
				t.Errorf("ValidateNamespace(%+v) errors = %v, want one error at %s", namespace, errs, tt.wantField)
			}
		})
	}
}

func TestValidateNamespaceUpdate(t *testing.T) {
	previous := &coreapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", UID: "uid", ResourceVersion: "1"}}
	next := previous.DeepCopy()
	next.Labels = map[string]string{"owner": "platform"}
	if errs := ValidateNamespaceUpdate(next, previous); len(errs) != 0 {
		t.Errorf("ValidateNamespaceUpdate(labels changed) errors = %v, want none", errs)
	}

	next.Name = "team-b"
	if errs := ValidateNamespaceUpdate(next, previous); len(errs) != 1 || errs[0].Field != "metadata.name" {
		t.Errorf("ValidateNamespaceUpdate(name changed) errors = %v, want immutable metadata.name error", errs)
	}
}

func TestValidateNamespaceMetadataFinalizers(t *testing.T) {
	previous := &coreapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a", ResourceVersion: "1"},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	next := previous.DeepCopy()
	next.Finalizers = []string{"example.com/cleanup"}
	if errs := ValidateNamespace(next); len(errs) != 0 {
		t.Errorf("ValidateNamespace(qualified metadata finalizer) errors = %v, want none", errs)
	}
	if errs := ValidateNamespaceUpdate(next, previous); len(errs) != 0 {
		t.Errorf("ValidateNamespaceUpdate(qualified metadata finalizer) errors = %v, want none", errs)
	}
	if errs := ValidateNamespaceStatusUpdate(next, previous); len(errs) != 0 {
		t.Errorf("ValidateNamespaceStatusUpdate(qualified metadata finalizer) errors = %v, want none", errs)
	}
	if errs := ValidateNamespaceFinalizeUpdate(next, previous); len(errs) != 0 {
		t.Errorf("ValidateNamespaceFinalizeUpdate(qualified metadata finalizer) errors = %v, want none", errs)
	}

	next.Finalizers = []string{"cleanup"}
	if errs := ValidateNamespace(next); len(errs) != 1 || errs[0].Field != "metadata.finalizers[0]" {
		t.Errorf("ValidateNamespace(bare metadata finalizer) errors = %v, want metadata.finalizers[0] error", errs)
	}
	if errs := ValidateNamespaceUpdate(next, previous); len(errs) != 1 || errs[0].Field != "metadata.finalizers[0]" {
		t.Errorf("ValidateNamespaceUpdate(bare metadata finalizer) errors = %v, want metadata.finalizers[0] error", errs)
	}
	if errs := ValidateNamespaceStatusUpdate(next, previous); len(errs) != 1 || errs[0].Field != "metadata.finalizers[0]" {
		t.Errorf("ValidateNamespaceStatusUpdate(bare metadata finalizer) errors = %v, want metadata.finalizers[0] error", errs)
	}
	if errs := ValidateNamespaceFinalizeUpdate(next, previous); len(errs) != 1 || errs[0].Field != "metadata.finalizers[0]" {
		t.Errorf("ValidateNamespaceFinalizeUpdate(bare metadata finalizer) errors = %v, want metadata.finalizers[0] error", errs)
	}
}

func TestValidateNamespaceStatusUpdate(t *testing.T) {
	deletedAt := metav1.NewTime(time.Unix(1, 0))
	tests := []struct {
		name      string
		phase     corev1.NamespacePhase
		deletedAt *metav1.Time
		wantErr   bool
	}{
		{name: "active", phase: corev1.NamespaceActive},
		{name: "terminating", phase: corev1.NamespaceTerminating, deletedAt: &deletedAt},
		{name: "premature terminating", phase: corev1.NamespaceTerminating, wantErr: true},
		{name: "active after deletion", phase: corev1.NamespaceActive, deletedAt: &deletedAt, wantErr: true},
		{name: "unknown phase", phase: "Unknown", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := &coreapi.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "team-a", ResourceVersion: "1", DeletionTimestamp: tt.deletedAt},
			}
			next := previous.DeepCopy()
			next.Status.Phase = tt.phase
			errs := ValidateNamespaceStatusUpdate(next, previous)
			if tt.wantErr {
				if len(errs) != 1 || errs[0].Field != "status.phase" {
					t.Errorf("ValidateNamespaceStatusUpdate(%s) errors = %v, want one status.phase error", tt.name, errs)
				}
				return
			}
			if len(errs) != 0 {
				t.Errorf("ValidateNamespaceStatusUpdate(%s) errors = %v, want none", tt.name, errs)
			}
		})
	}
}

func TestValidateNamespaceFinalizeUpdate(t *testing.T) {
	previous := &coreapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a", ResourceVersion: "1"},
		Spec:       corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{corev1.FinalizerKubernetes}},
	}
	next := previous.DeepCopy()
	next.Spec.Finalizers = nil
	if errs := ValidateNamespaceFinalizeUpdate(next, previous); len(errs) != 0 {
		t.Errorf("ValidateNamespaceFinalizeUpdate(remove finalizer) errors = %v, want none", errs)
	}

	next.Spec.Finalizers = []corev1.FinalizerName{"example.com/cleanup", "cleanup"}
	if errs := ValidateNamespaceFinalizeUpdate(next, previous); len(errs) != 1 || errs[0].Field != "spec.finalizers[1]" {
		t.Errorf("ValidateNamespaceFinalizeUpdate(bare custom finalizer) errors = %v, want error at spec.finalizers[1]", errs)
	}
}
