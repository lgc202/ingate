package reconcile

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
)

type eventRegistration struct {
	name     string
	informer cache.SharedIndexInformer
}

func (r *Reconciler) registerEventHandlers(registrations []eventRegistration) error {
	handler := r.eventHandler()
	for _, registration := range registrations {
		if _, err := registration.informer.AddEventHandler(handler); err != nil {
			return fmt.Errorf("register %s informer handler: %w", registration.name, err)
		}
	}
	return nil
}

func (r *Reconciler) eventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(any) {
			r.queue.Add(queueKeyConfig)
		},
		UpdateFunc: func(oldObject, newObject any) {
			oldMetadata, oldErr := meta.Accessor(oldObject)
			newMetadata, newErr := meta.Accessor(newObject)
			if oldErr == nil && newErr == nil && oldMetadata.GetGeneration() == newMetadata.GetGeneration() {
				return
			}
			r.queue.Add(queueKeyConfig)
		},
		DeleteFunc: func(any) {
			r.queue.Add(queueKeyConfig)
		},
	}
}
