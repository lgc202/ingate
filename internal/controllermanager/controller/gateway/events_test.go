package gateway

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestAIConfigPredicateScope(t *testing.T) {
	writer := aiConfigWriter{namespace: "control-plane"}
	filter := writer.predicate()
	tests := []struct {
		name      string
		namespace string
		object    string
		want      bool
	}{
		{name: "target", namespace: "control-plane", object: aidataplane.ConfigMapName, want: true},
		{name: "other namespace", namespace: "team-a", object: aidataplane.ConfigMapName},
		{name: "other ConfigMap", namespace: "control-plane", object: "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: tt.namespace, Name: tt.object}}
			current := previous.DeepCopy()
			current.Data = map[string]string{aidataplane.ConfigMapDataKey: "changed"}

			if got := filter.Create(event.CreateEvent{Object: previous}); got != tt.want {
				t.Errorf("predicate.Create(%s) = %t, want %t", tt.name, got, tt.want)
			}

			if got := filter.Delete(event.DeleteEvent{Object: previous}); got != tt.want {
				t.Errorf("predicate.Delete(%s) = %t, want %t", tt.name, got, tt.want)
			}

			if got := filter.Update(event.UpdateEvent{ObjectOld: previous, ObjectNew: current}); got != tt.want {
				t.Errorf("predicate.Update(%s) = %t, want %t", tt.name, got, tt.want)
			}

			if filter.Generic(event.GenericEvent{Object: current}) {
				t.Error("predicate.Generic() = true, want false")
			}
		})
	}
}

func TestAIConfigPredicateUpdates(t *testing.T) {
	writer := aiConfigWriter{namespace: "control-plane"}
	filter := writer.predicate()
	tests := []struct {
		name   string
		change func(*corev1.ConfigMap)
		want   bool
	}{
		{name: "unchanged", change: func(*corev1.ConfigMap) {}},
		{name: "resourceVersion only", change: func(object *corev1.ConfigMap) { object.ResourceVersion = "43" }},
		{name: "annotation", change: func(object *corev1.ConfigMap) { object.Annotations = map[string]string{"other": "new"} }},
		{name: "other data key", change: func(object *corev1.ConfigMap) { object.Data["other"] = "new" }},
		{name: "other binary key", change: func(object *corev1.ConfigMap) { object.BinaryData = map[string][]byte{"other": {1}} }},
		{name: "owned payload", change: func(object *corev1.ConfigMap) { object.Data[aidataplane.ConfigMapDataKey] = "new" }, want: true},
		{name: "removed payload", change: func(object *corev1.ConfigMap) { delete(object.Data, aidataplane.ConfigMapDataKey) }, want: true},
		{name: "removed management label", change: func(object *corev1.ConfigMap) { object.Labels = nil }, want: true},
		{name: "different management label", change: func(object *corev1.ConfigMap) { object.Labels[aiConfigManagedByKey] = "other" }, want: true},
		{name: "binary key occupied even when empty", change: func(object *corev1.ConfigMap) {
			object.BinaryData = map[string][]byte{aidataplane.ConfigMapDataKey: {}}
		}, want: true},
		{name: "immutable", change: func(object *corev1.ConfigMap) { object.Immutable = new(true) }, want: true},
		{name: "deletion starts", change: func(object *corev1.ConfigMap) { object.DeletionTimestamp = new(metav1.Unix(3, 0)) }, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := managedAIConfigMap("old")
			current := previous.DeepCopy()
			tt.change(current)

			got := filter.Update(event.UpdateEvent{ObjectOld: &previous, ObjectNew: current})

			if got != tt.want {
				t.Errorf("predicate.Update(%s) = %t, want %t", tt.name, got, tt.want)
			}
		})
	}
}
