package shared

import (
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type GatewayKeyResolver func(obj interface{}) []ObjectKey

func NewGatewayEventHandler(resolver GatewayKeyResolver, queue workqueue.TypedRateLimitingInterface[ObjectKey]) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			enqueueResolvedGatewayKeys(resolver, queue, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			seen := make(map[string]struct{})
			enqueueResolvedGatewayKeysWithSeen(resolver, queue, oldObj, seen)
			enqueueResolvedGatewayKeysWithSeen(resolver, queue, newObj, seen)
		},
		DeleteFunc: func(obj interface{}) {
			enqueueResolvedGatewayKeys(resolver, queue, obj)
		},
	}
}

func enqueueResolvedGatewayKeys(resolver GatewayKeyResolver, queue workqueue.TypedRateLimitingInterface[ObjectKey], obj interface{}) {
	enqueueResolvedGatewayKeysWithSeen(resolver, queue, obj, nil)
}

func enqueueResolvedGatewayKeysWithSeen(resolver GatewayKeyResolver, queue workqueue.TypedRateLimitingInterface[ObjectKey], obj interface{}, seen map[string]struct{}) {
	if resolver == nil || queue == nil {
		return
	}

	obj = unwrapDeletedFinalStateUnknown(obj)
	if obj == nil {
		return
	}

	for _, key := range resolver(obj) {
		if key.Name == "" {
			continue
		}
		serialized := key.String()
		if seen != nil {
			if _, ok := seen[serialized]; ok {
				continue
			}
			seen[serialized] = struct{}{}
		}
		queue.Add(key)
	}
}

func unwrapDeletedFinalStateUnknown(obj interface{}) interface{} {
	switch typed := obj.(type) {
	case cache.DeletedFinalStateUnknown:
		return typed.Obj
	case *cache.DeletedFinalStateUnknown:
		if typed == nil {
			return nil
		}
		return typed.Obj
	default:
		return obj
	}
}
