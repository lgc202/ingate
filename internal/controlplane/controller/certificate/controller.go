package certificate

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
	queue    *shared.GatewayQueue
}

func NewController(ctx *controllerruntime.Context) *Controller {
	if ctx == nil {
		return &Controller{}
	}
	var informer cache.SharedIndexInformer
	if ctx.InformerFactory != nil {
		informer = ctx.InformerFactory.Gateway().V1alpha1().Certificates().Informer()
	}
	return &Controller{informer: informer, index: ctx.Index, queue: ctx.GatewayQueue}
}

func (c *Controller) Name() string { return names.CertificateControllerName }

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
	certificate, ok := asCertificate(obj)
	if !ok || c.index == nil {
		return
	}
	c.index.UpsertCertificate(certificate)
	c.enqueue(c.index.AffectedGatewaysForCertificate(objectKeyOf(certificate)))
}

func (c *Controller) onUpdate(oldObj, newObj interface{}) {
	oldCertificate, ok := asCertificate(oldObj)
	if !ok || c.index == nil {
		return
	}
	newCertificate, ok := asCertificate(newObj)
	if !ok {
		return
	}
	oldKey := objectKeyOf(oldCertificate)
	oldAffected := c.index.AffectedGatewaysForCertificate(oldKey)
	c.index.UpsertCertificate(newCertificate)
	newAffected := c.index.AffectedGatewaysForCertificate(objectKeyOf(newCertificate))
	c.enqueue(append(oldAffected, newAffected...))
}

func (c *Controller) onDelete(obj interface{}) {
	certificate, ok := asCertificate(obj)
	if !ok || c.index == nil {
		return
	}
	key := objectKeyOf(certificate)
	oldAffected := c.index.AffectedGatewaysForCertificate(key)
	c.index.DeleteCertificate(key)
	c.enqueue(oldAffected)
}

func (c *Controller) enqueue(keys []shared.ObjectKey) {
	if c == nil || c.queue == nil {
		return
	}
	seen := map[string]struct{}{}
	for _, key := range keys {
		if key.Name == "" {
			continue
		}
		if _, ok := seen[key.String()]; ok {
			continue
		}
		seen[key.String()] = struct{}{}
		c.queue.Enqueue(key)
	}
}

func asCertificate(obj interface{}) (*gatewayv1alpha1.Certificate, bool) {
	obj = unwrapDeletedFinalStateUnknown(obj)
	certificate, ok := obj.(*gatewayv1alpha1.Certificate)
	return certificate, ok
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
