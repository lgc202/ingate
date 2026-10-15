package gateway

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// resourcePredicate 忽略 Status 和无关 metadata 更新，但保留带 finalizer 的删除开始事件。
func resourcePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(change event.UpdateEvent) bool {
			return change.ObjectOld.GetGeneration() != change.ObjectNew.GetGeneration() ||
				change.ObjectOld.GetDeletionTimestamp().IsZero() != change.ObjectNew.GetDeletionTimestamp().IsZero()
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// predicate 不用 managed-by 做筛选，否则标签被移除后无法触发冲突诊断。
// 只关注所属内容与影响写入的字段，忽略自身写入带来的纯 resourceVersion 变化。
func (w *aiConfigWriter) predicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(change event.CreateEvent) bool { return w.matches(change.Object) },
		DeleteFunc: func(change event.DeleteEvent) bool { return w.matches(change.Object) },
		UpdateFunc: func(change event.UpdateEvent) bool {
			if !w.matches(change.ObjectNew) {
				return false
			}

			previous, previousOK := change.ObjectOld.(*corev1.ConfigMap)
			current, currentOK := change.ObjectNew.(*corev1.ConfigMap)
			if !previousOK || !currentOK {
				return false
			}

			_, previousOccupied := previous.BinaryData[aidataplane.ConfigMapDataKey]
			_, currentOccupied := current.BinaryData[aidataplane.ConfigMapDataKey]

			return previous.Data[aidataplane.ConfigMapDataKey] != current.Data[aidataplane.ConfigMapDataKey] ||
				previous.Labels[aiConfigManagedByKey] != current.Labels[aiConfigManagedByKey] ||
				previous.GetDeletionTimestamp().IsZero() != current.GetDeletionTimestamp().IsZero() ||
				ptr.Deref(previous.Immutable, false) != ptr.Deref(current.Immutable, false) ||
				previousOccupied != currentOccupied
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

func (w *aiConfigWriter) matches(object client.Object) bool {
	return object != nil && object.GetNamespace() == w.namespace && object.GetName() == aidataplane.ConfigMapName
}
