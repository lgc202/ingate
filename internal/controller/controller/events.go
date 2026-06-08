package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

type eventRegistration struct {
	informer cache.SharedIndexInformer
	handler  cache.ResourceEventHandlerFuncs
}

func (c *Controller) registerEventHandlers() error {
	gatewayInformers := c.factory.Gateway().V1()
	registrations := []eventRegistration{
		{informer: gatewayInformers.Gateways().Informer(), handler: c.eventHandler(c.enqueueGatewayObject)},
		{informer: gatewayInformers.Routes().Informer(), handler: c.eventHandler(c.enqueueRouteObject)},
		{informer: gatewayInformers.Upstreams().Informer(), handler: c.eventHandler(c.enqueueUpstreamObject)},
		{
			informer: gatewayInformers.AuthPolicies().Informer(),
			handler: c.eventHandler(func(obj any) {
				c.enqueuePolicyObject(resource.KindAuthPolicy, obj)
			}),
		},
		{
			informer: gatewayInformers.RateLimitPolicies().Informer(),
			handler: c.eventHandler(func(obj any) {
				c.enqueuePolicyObject(resource.KindRateLimitPolicy, obj)
			}),
		},
		{informer: gatewayInformers.RedisStores().Informer(), handler: c.eventHandler(c.enqueueRedisStoreObject)},
		{informer: gatewayInformers.PolicyBindings().Informer(), handler: c.eventHandler(c.enqueuePolicyBindingObject)},
	}

	for _, registration := range registrations {
		if _, err := registration.informer.AddEventHandler(registration.handler); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) eventHandler(enqueue func(any)) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: enqueue,
		UpdateFunc: func(oldObj, newObj any) {
			if sameResourceVersion(oldObj, newObj) {
				return
			}
			enqueue(newObj)
		},
		DeleteFunc: enqueue,
	}
}

func sameResourceVersion(oldObj, newObj any) bool {
	oldMeta, err := meta.Accessor(oldObj)
	if err != nil {
		return false
	}
	newMeta, err := meta.Accessor(newObj)
	if err != nil {
		return false
	}
	oldResourceVersion := oldMeta.GetResourceVersion()
	return oldResourceVersion != "" && oldResourceVersion == newMeta.GetResourceVersion()
}

func (c *Controller) waitForCacheSync(ctx context.Context) error {
	for _, synced := range c.factory.WaitForCacheSync(ctx.Done()) {
		if synced {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("cache sync failed")
	}
	return nil
}

func (c *Controller) enqueueAllGateways() error {
	gateways, err := c.gatewayLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, gateway := range gateways {
		c.enqueueGateway(gateway.Name)
	}
	return nil
}

func (c *Controller) enqueueGatewayObject(obj any) {
	gateway, ok := objectAs[*resource.Gateway](obj)
	if !ok {
		return
	}
	c.enqueueGateway(gateway.Name)
}

func (c *Controller) enqueueRouteObject(obj any) {
	route, ok := objectAs[*resource.Route](obj)
	if !ok {
		return
	}
	for _, parentRef := range route.Spec.ParentRefs {
		c.enqueueGateway(parentRef.Name)
	}
}

func (c *Controller) enqueueUpstreamObject(obj any) {
	upstream, ok := objectAs[*resource.Upstream](obj)
	if !ok {
		return
	}

	routes, err := c.routesByIndex(routeIndexUpstreamRef, upstream.Name)
	if err != nil {
		return
	}
	for _, route := range routes {
		for _, parentRef := range route.Spec.ParentRefs {
			c.enqueueGateway(parentRef.Name)
		}
	}
}

func (c *Controller) enqueuePolicyObject(kind resource.Kind, obj any) {
	name, ok := policyObjectName(obj)
	if !ok {
		return
	}

	bindings, err := c.policyBindingsByIndex(policyBindingIndexPolicy, targetIndexValue(kind, name))
	if err != nil {
		return
	}
	for _, binding := range bindings {
		c.enqueuePolicyBinding(binding)
	}
}

func (c *Controller) enqueueRedisStoreObject(obj any) {
	store, ok := objectAs[*resource.RedisStore](obj)
	if !ok {
		return
	}

	policies, err := c.rateLimitPoliciesByIndex(rateLimitPolicyIndexRedis, store.Name)
	if err != nil {
		return
	}
	for _, policy := range policies {
		c.enqueuePolicyByName(resource.KindRateLimitPolicy, policy.Name)
	}
}

func (c *Controller) enqueuePolicyByName(kind resource.Kind, name string) {
	bindings, err := c.policyBindingsByIndex(policyBindingIndexPolicy, targetIndexValue(kind, name))
	if err != nil {
		return
	}
	for _, binding := range bindings {
		c.enqueuePolicyBinding(binding)
	}
}

func (c *Controller) enqueuePolicyBindingObject(obj any) {
	binding, ok := objectAs[*resource.PolicyBinding](obj)
	if !ok {
		return
	}

	c.enqueuePolicyBinding(binding)
}

func (c *Controller) enqueuePolicyBinding(binding *resource.PolicyBinding) {
	// PolicyBinding 只影响 Gateway/Route 这条 API 网关链路
	c.enqueueGatewayByTarget(binding.Spec.TargetRef.Kind, binding.Spec.TargetRef.Name)
}

func (c *Controller) enqueueGatewayByTarget(kind resource.Kind, name string) {
	switch kind {
	case resource.KindGateway:
		c.enqueueGateway(name)
	case resource.KindRoute:
		route, err := c.routeLister.Get(name)
		if err != nil {
			return
		}
		for _, parentRef := range route.Spec.ParentRefs {
			c.enqueueGateway(parentRef.Name)
		}
	case resource.KindUpstream:
		routes, err := c.routesByIndex(routeIndexUpstreamRef, name)
		if err != nil {
			return
		}
		for _, route := range routes {
			for _, parentRef := range route.Spec.ParentRefs {
				c.enqueueGateway(parentRef.Name)
			}
		}
	}
}

func (c *Controller) enqueueGateway(name string) {
	if name == "" {
		return
	}
	c.queue.Add(name)
}

func (c *Controller) routesByIndex(index routeIndexName, value string) ([]*resource.Route, error) {
	items, err := c.routeIndexer.ByIndex(string(index), value)
	if err != nil {
		return nil, err
	}

	routes := make([]*resource.Route, 0, len(items))
	for _, item := range items {
		route, ok := item.(*resource.Route)
		if !ok {
			continue
		}
		routes = append(routes, route)
	}
	return routes, nil
}

func (c *Controller) rateLimitPoliciesByIndex(index routeIndexName, value string) ([]*resource.RateLimitPolicy, error) {
	items, err := c.rateLimitIndexer.ByIndex(string(index), value)
	if err != nil {
		return nil, err
	}

	policies := make([]*resource.RateLimitPolicy, 0, len(items))
	for _, item := range items {
		policy, ok := item.(*resource.RateLimitPolicy)
		if !ok {
			continue
		}
		policies = append(policies, policy)
	}
	return policies, nil
}

func (c *Controller) policyBindingsByIndex(index routeIndexName, value string) ([]*resource.PolicyBinding, error) {
	items, err := c.policyBindingIndexer.ByIndex(string(index), value)
	if err != nil {
		return nil, err
	}

	bindings := make([]*resource.PolicyBinding, 0, len(items))
	for _, item := range items {
		binding, ok := item.(*resource.PolicyBinding)
		if !ok {
			continue
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func objectAs[T any](obj any) (T, bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	value, ok := obj.(T)
	return value, ok
}

func policyObjectName(obj any) (string, bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	switch policy := obj.(type) {
	case *resource.AuthPolicy:
		return policy.Name, true
	case *resource.RateLimitPolicy:
		return policy.Name, true
	default:
		return "", false
	}
}

func routeParentRefIndex(obj any) ([]string, error) {
	route, ok := obj.(*resource.Route)
	if !ok {
		return nil, nil
	}
	parentRefs := make([]string, 0, len(route.Spec.ParentRefs))
	for _, parentRef := range route.Spec.ParentRefs {
		parentRefs = append(parentRefs, parentRef.Name)
	}
	return uniqueStrings(parentRefs), nil
}

func routeUpstreamRefIndex(obj any) ([]string, error) {
	route, ok := obj.(*resource.Route)
	if !ok {
		return nil, nil
	}

	upstreamNames := make([]string, 0)
	for _, rule := range route.Spec.Rules {
		for _, upstreamRef := range rule.UpstreamRefs {
			upstreamNames = append(upstreamNames, upstreamRef.Name)
		}
	}
	return uniqueStrings(upstreamNames), nil
}

func policyBindingTargetRefIndex(obj any) ([]string, error) {
	binding, ok := obj.(*resource.PolicyBinding)
	if !ok {
		return nil, nil
	}
	return uniqueStrings([]string{targetIndexValue(binding.Spec.TargetRef.Kind, binding.Spec.TargetRef.Name)}), nil
}

func rateLimitPolicyRedisRefIndex(obj any) ([]string, error) {
	policy, ok := obj.(*resource.RateLimitPolicy)
	if !ok || policy.Spec.Global == nil {
		return nil, nil
	}
	return uniqueStrings([]string{policy.Spec.Global.RedisRef}), nil
}

func policyBindingPolicyRefIndex(obj any) ([]string, error) {
	binding, ok := obj.(*resource.PolicyBinding)
	if !ok {
		return nil, nil
	}

	policyNames := make([]string, 0, len(binding.Spec.Policies))
	for _, policyRef := range binding.Spec.Policies {
		policyNames = append(policyNames, targetIndexValue(policyRef.Kind, policyRef.Name))
	}
	return uniqueStrings(policyNames), nil
}

func targetIndexValue(kind resource.Kind, name string) string {
	return string(kind) + indexTargetSeparator + name
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
