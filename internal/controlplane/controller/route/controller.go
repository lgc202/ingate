package route

import (
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/lgc202/ingate/cmd/controller-manager/names"
	controllerindex "github.com/lgc202/ingate/internal/controlplane/controller/index"
	controllerruntime "github.com/lgc202/ingate/internal/controlplane/controller/runtime"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
)

type Controller struct {
	informer cache.SharedIndexInformer
	index    *controllerindex.Index
	queue    workqueue.TypedRateLimitingInterface[shared.ObjectKey]
}

func NewController(ctx *controllerruntime.Context) *Controller {
	if ctx == nil {
		return &Controller{}
	}

	var informer cache.SharedIndexInformer
	if ctx.InformerFactory != nil {
		informer = ctx.InformerFactory.Gateway().V1alpha1().Routes().Informer()
	}

	return &Controller{informer: informer, index: ctx.Index, queue: ctx.GatewayQueue}
}

func (c *Controller) Name() string { return names.RouteControllerName }

func (c *Controller) Register() error {
	if c == nil || c.informer == nil {
		return nil
	}

	_, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.onAdd(obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { c.onUpdate(oldObj, newObj) },
		DeleteFunc: func(obj interface{}) { c.onDelete(obj) },
	})
	return err
}

func (c *Controller) onAdd(obj interface{}) {
	route, ok := asRoute(obj)
	if !ok || c.index == nil {
		return
	}
	c.index.UpsertRoute(route)
	c.enqueue(c.index.AffectedGatewaysForRoute(objectKeyOf(route)))
}

func (c *Controller) onUpdate(oldObj, newObj interface{}) {
	oldRoute, ok := asRoute(oldObj)
	if !ok || c.index == nil {
		return
	}
	newRoute, ok := asRoute(newObj)
	if !ok {
		return
	}

	oldKey := objectKeyOf(oldRoute)
	oldAffected := c.index.AffectedGatewaysForRoute(oldKey)

	c.index.UpsertRoute(newRoute)
	newAffected := c.index.AffectedGatewaysForRoute(objectKeyOf(newRoute))
	c.enqueue(append(oldAffected, newAffected...))
}

func (c *Controller) onDelete(obj interface{}) {
	route, ok := asRoute(obj)
	if !ok || c.index == nil {
		return
	}
	key := objectKeyOf(route)
	oldAffected := c.index.AffectedGatewaysForRoute(key)
	c.index.DeleteRoute(key)
	c.enqueue(oldAffected)
}

func (c *Controller) enqueue(keys []shared.ObjectKey) {
	if c == nil || c.queue == nil {
		return
	}
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key.Name == "" {
			continue
		}
		serialized := key.String()
		if _, ok := seen[serialized]; ok {
			continue
		}
		seen[serialized] = struct{}{}
		c.queue.Add(key)
	}
}

func asRoute(obj interface{}) (*gatewayv1alpha1.Route, bool) {
	obj = unwrapDeletedFinalStateUnknown(obj)
	route, ok := obj.(*gatewayv1alpha1.Route)
	return route, ok
}

func objectKeyOf(obj interface {
	GetNamespace() string
	GetName() string
}) shared.ObjectKey {
	return shared.NewObjectKey(obj.GetNamespace(), obj.GetName())
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
