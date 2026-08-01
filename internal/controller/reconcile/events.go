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

func (c *resourceCache) registerEventHandlers(registrations []eventRegistration) error {
	handler := c.eventHandler()
	for _, registration := range registrations {
		if _, err := registration.informer.AddEventHandler(handler); err != nil {
			return fmt.Errorf("register %s informer handler: %w", registration.name, err)
		}
	}
	return nil
}

func (c *resourceCache) eventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(any) {
			c.onDesiredChange()
		},
		UpdateFunc: func(oldObject, newObject any) {
			oldMetadata, oldErr := meta.Accessor(oldObject)
			newMetadata, newErr := meta.Accessor(newObject)
			// Status 更新不会推进 Generation，忽略它可以避免状态回写再次触发全量编译
			if oldErr == nil && newErr == nil && oldMetadata.GetGeneration() == newMetadata.GetGeneration() {
				return
			}
			c.onDesiredChange()
		},
		DeleteFunc: func(any) {
			c.onDesiredChange()
		},
	}
}
