package apiserver

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
)

type eventRegistration struct {
	name     string
	informer cache.SharedIndexInformer
}

func (w *ResourceWatcher) registerEventHandlers(registrations []eventRegistration) error {
	handler := w.eventHandler()
	for _, registration := range registrations {
		if _, err := registration.informer.AddEventHandler(handler); err != nil {
			return fmt.Errorf("register %s informer handler: %w", registration.name, err)
		}
	}
	return nil
}

func (w *ResourceWatcher) eventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(any) {
			w.notifyChange()
		},
		UpdateFunc: func(oldObject, newObject any) {
			oldMetadata, oldErr := meta.Accessor(oldObject)
			newMetadata, newErr := meta.Accessor(newObject)
			// Status 更新不会推进 Generation，忽略它可以避免状态回写再次触发全量编译
			if oldErr == nil && newErr == nil && oldMetadata.GetGeneration() == newMetadata.GetGeneration() {
				return
			}
			w.notifyChange()
		},
		DeleteFunc: func(any) {
			w.notifyChange()
		},
	}
}

func (w *ResourceWatcher) notifyChange() {
	select {
	case w.changes <- struct{}{}:
	default:
	}
}
