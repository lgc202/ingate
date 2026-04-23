package gateway

import (
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	"k8s.io/client-go/tools/cache"

	"github.com/lgc202/ingate/cmd/controller-manager/names"
	controllerindex "github.com/lgc202/ingate/internal/controlplane/controller/index"
	controllerruntime "github.com/lgc202/ingate/internal/controlplane/controller/runtime"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
)

type Controller struct {
	informer cache.SharedIndexInformer
	index    *controllerindex.Index
	queue    shared.GatewayQueue
}

func NewController(ctx *controllerruntime.Context) *Controller {
	if ctx == nil {
		return &Controller{}
	}
	var informer cache.SharedIndexInformer
	if ctx.InformerFactory != nil {
		informer = ctx.InformerFactory.Gateway().V1alpha1().Gateways().Informer()
	}
	return &Controller{informer: informer, index: ctx.Index, queue: ctx.GatewayQueue}
}

func (c *Controller) Name() string { return names.GatewayControllerName }

func (c *Controller) Register() error {
	if c == nil || c.informer == nil {
		return nil
	}
	_, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.onAdd(obj) },
		UpdateFunc: func(_, newObj interface{}) { c.onAdd(newObj) },
		DeleteFunc: func(obj interface{}) { c.onDelete(obj) },
	})
	return err
}

func (c *Controller) onAdd(obj interface{}) {
	gateway, ok := asGateway(obj)
	if !ok || c.index == nil {
		return
	}
	key := objectKeyOf(gateway)
	c.index.UpsertGateway(gateway)
	if c.queue != nil {
		c.queue.Enqueue(key)
	}
}

func (c *Controller) onDelete(obj interface{}) {
	gateway, ok := asGateway(obj)
	if !ok || c.index == nil {
		return
	}
	key := objectKeyOf(gateway)
	c.index.DeleteGateway(key)
	if c.queue != nil {
		c.queue.Enqueue(key)
	}
}

func asGateway(obj interface{}) (*gatewayv1alpha1.Gateway, bool) {
	obj = unwrapDeletedFinalStateUnknown(obj)
	gateway, ok := obj.(*gatewayv1alpha1.Gateway)
	return gateway, ok
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
