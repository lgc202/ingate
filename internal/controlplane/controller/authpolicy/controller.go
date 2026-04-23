package authpolicy

import (
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
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
		informer = ctx.InformerFactory.Policy().V1alpha1().AuthPolicies().Informer()
	}
	return &Controller{informer: informer, index: ctx.Index, queue: ctx.GatewayQueue}
}

func (c *Controller) Name() string { return names.AuthPolicyControllerName }

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
	policy, ok := asAuthPolicy(obj)
	if !ok || c.index == nil {
		return
	}
	c.index.UpsertAuthPolicy(policy)
	c.enqueue(c.index.AffectedGatewaysForAuthPolicy(objectKeyOf(policy)))
}

func (c *Controller) onUpdate(oldObj, newObj interface{}) {
	oldPolicy, ok := asAuthPolicy(oldObj)
	if !ok || c.index == nil {
		return
	}
	newPolicy, ok := asAuthPolicy(newObj)
	if !ok {
		return
	}
	oldKey := objectKeyOf(oldPolicy)
	oldAffected := c.index.AffectedGatewaysForAuthPolicy(oldKey)
	c.index.UpsertAuthPolicy(newPolicy)
	newAffected := c.index.AffectedGatewaysForAuthPolicy(objectKeyOf(newPolicy))
	c.enqueue(append(oldAffected, newAffected...))
}

func (c *Controller) onDelete(obj interface{}) {
	policy, ok := asAuthPolicy(obj)
	if !ok || c.index == nil {
		return
	}
	key := objectKeyOf(policy)
	oldAffected := c.index.AffectedGatewaysForAuthPolicy(key)
	c.index.DeleteAuthPolicy(key)
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

func asAuthPolicy(obj interface{}) (*policyv1alpha1.AuthPolicy, bool) {
	obj = unwrapDeletedFinalStateUnknown(obj)
	policy, ok := obj.(*policyv1alpha1.AuthPolicy)
	return policy, ok
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
