package namespace

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

func TestShouldDeleteDuringUpdate(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name      string
		namespace coreapi.Namespace
		want      bool
	}{
		{
			name:      "finalized",
			namespace: coreapi.Namespace{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}},
			want:      true,
		},
		{name: "not_deleting"},
		{
			name: "spec_finalizer",
			namespace: coreapi.Namespace{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
				Spec:       corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{corev1.FinalizerKubernetes}},
			},
		},
		{
			name: "metadata_finalizer",
			namespace: coreapi.Namespace{ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &now,
				Finalizers:        []string{"example.com/cleanup"},
			}},
		},
		{
			name: "grace_period",
			namespace: coreapi.Namespace{ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp:          &now,
				DeletionGracePeriodSeconds: ptr.To(int64(10)),
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldDeleteDuringUpdate(t.Context(), "key", &tt.namespace, &tt.namespace); got != tt.want {
				t.Errorf("shouldDeleteDuringUpdate() = %t, want %t", got, tt.want)
			}
		})
	}
	if shouldDeleteDuringUpdate(t.Context(), "key", &coreapi.NamespaceList{}, &coreapi.Namespace{}) {
		t.Error("shouldDeleteDuringUpdate(NamespaceList) = true, want false")
	}
}

func TestSetPropagationFinalizers(t *testing.T) {
	const cleanup = "example.com/cleanup"
	tests := []struct {
		name       string
		finalizers []string
		options    metav1.DeleteOptions
		want       []string
	}{
		{
			name:       "unspecified",
			finalizers: []string{metav1.FinalizerOrphanDependents, cleanup},
			want:       []string{metav1.FinalizerOrphanDependents, cleanup},
		},
		{
			name:       "orphan",
			finalizers: []string{cleanup, metav1.FinalizerDeleteDependents},
			options:    metav1.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationOrphan)},
			want:       []string{cleanup, metav1.FinalizerOrphanDependents},
		},
		{
			name:       "foreground",
			finalizers: []string{metav1.FinalizerOrphanDependents, cleanup},
			options:    metav1.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationForeground)},
			want:       []string{cleanup, metav1.FinalizerDeleteDependents},
		},
		{
			name:       "background",
			finalizers: []string{metav1.FinalizerOrphanDependents, cleanup, metav1.FinalizerDeleteDependents},
			options:    metav1.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationBackground)},
			want:       []string{cleanup},
		},
		{
			name:       "deprecated_orphan_priority",
			finalizers: []string{cleanup},
			options: metav1.DeleteOptions{
				//nolint:staticcheck // 验证标准 DeleteOptions 中弃用字段的既有优先级。
				OrphanDependents:  ptr.To(true),
				PropagationPolicy: ptr.To(metav1.DeletePropagationForeground),
			},
			want: []string{cleanup, metav1.FinalizerOrphanDependents},
		},
		{
			name:       "deprecated_foreground",
			finalizers: []string{cleanup},
			//nolint:staticcheck // 验证标准 DeleteOptions 中尚保留的 false 分支。
			options: metav1.DeleteOptions{OrphanDependents: ptr.To(false)},
			want:    []string{cleanup, metav1.FinalizerDeleteDependents},
		},
		{
			name:       "keep_order",
			finalizers: []string{metav1.FinalizerOrphanDependents, cleanup, "example.com/second"},
			options:    metav1.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationOrphan)},
			want:       []string{metav1.FinalizerOrphanDependents, cleanup, "example.com/second"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := &coreapi.Namespace{ObjectMeta: metav1.ObjectMeta{Finalizers: tt.finalizers}}
			optionsBefore := tt.options.DeepCopy()
			setPropagationFinalizers(namespace, &tt.options)
			if diff := cmp.Diff(tt.want, namespace.Finalizers); diff != "" {
				t.Errorf("propagation finalizers mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(optionsBefore, &tt.options); diff != "" {
				t.Errorf("propagation mutated options (-want +got):\n%s", diff)
			}
		})
	}
}
