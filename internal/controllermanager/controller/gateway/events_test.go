package gateway

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestResourcePredicate(t *testing.T) {
	filter := resourcePredicate()
	for _, resource := range []client.Object{
		&gatewayv1alpha1.Gateway{}, &gatewayv1alpha1.Backend{}, &gatewayv1alpha1.HTTPRoute{},
		&gatewayv1alpha1.AIBackend{}, &gatewayv1alpha1.ModelRoute{}, &gatewayv1alpha1.MCPRoute{},
	} {
		resource.SetGeneration(1)
		if !filter.Create(event.CreateEvent{Object: resource}) || !filter.Delete(event.DeleteEvent{Object: resource}) {
			t.Errorf("resourcePredicate(%T) filtered a create or delete event", resource)
		}

		if filter.Generic(event.GenericEvent{Object: resource}) {
			t.Errorf("resourcePredicate(%T) accepted a generic event", resource)
		}

		for _, tt := range []struct {
			name   string
			change func(client.Object)
			want   bool
		}{
			{name: "unchanged", change: func(client.Object) {}},
			{name: "resourceVersion only", change: func(object client.Object) { object.SetResourceVersion("2") }},
			{name: "annotation", change: func(object client.Object) { object.SetAnnotations(map[string]string{"other": "new"}) }},
			{name: "finalizer only", change: func(object client.Object) { object.SetFinalizers([]string{"example.com/cleanup"}) }},
			{name: "spec generation", change: func(object client.Object) { object.SetGeneration(2) }, want: true},
			{name: "deletion starts", change: func(object client.Object) { object.SetDeletionTimestamp(new(metav1.Unix(3, 0))) }, want: true},
		} {
			current := resource.DeepCopyObject().(client.Object)
			tt.change(current)

			got := filter.Update(event.UpdateEvent{ObjectOld: resource, ObjectNew: current})

			if got != tt.want {
				t.Errorf("resourcePredicate.Update(%T, %s) = %t, want %t", resource, tt.name, got, tt.want)
			}
		}
	}

	previous := &gatewayv1alpha1.Gateway{}
	current := previous.DeepCopy()
	current.Status.Conditions = []metav1.Condition{{Type: gatewayv1alpha1.ConditionAccepted, Status: metav1.ConditionTrue}}

	if filter.Update(event.UpdateEvent{ObjectOld: previous, ObjectNew: current}) {
		t.Error("resourcePredicate.Update(status only) = true, want false")
	}
}

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
