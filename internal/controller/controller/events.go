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
		{informer: gatewayInformers.AIRoutes().Informer(), handler: c.eventHandler(c.enqueueAIRouteObject)},
		{informer: gatewayInformers.AIProviders().Informer(), handler: c.eventHandler(c.enqueueAIProviderObject)},
		{informer: gatewayInformers.AIModels().Informer(), handler: c.eventHandler(c.enqueueAIModelObject)},
		{informer: gatewayInformers.AIPolicies().Informer(), handler: c.eventHandler(c.enqueueAIPolicyObject)},
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
		{informer: gatewayInformers.PolicyBindings().Informer(), handler: c.eventHandler(c.enqueuePolicyBindingObject)},
		{informer: gatewayInformers.Plugins().Informer(), handler: c.eventHandler(c.enqueuePluginObject)},
		{informer: gatewayInformers.PluginBindings().Informer(), handler: c.eventHandler(c.enqueuePluginBindingObject)},
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
		c.enqueueGateway(parentRef)
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
			c.enqueueGateway(parentRef)
		}
	}
}

func (c *Controller) enqueueAIRouteObject(obj any) {
	route, ok := objectAs[*resource.AIRoute](obj)
	if !ok {
		return
	}
	for _, parentRef := range route.Spec.ParentRefs {
		c.enqueueGateway(parentRef)
	}
}

func (c *Controller) enqueueAIProviderObject(obj any) {
	provider, ok := objectAs[*resource.AIProvider](obj)
	if !ok {
		return
	}

	c.enqueueAIProvider(provider.Name)
}

func (c *Controller) enqueueAIProvider(name string) {
	// AIProvider 可以被 AIRoute 直接引用，也可以被 AIModel 间接引用
	// Provider 变化时要沿着这两条路径找到受影响的 Gateway，避免回退到全量 reconcile
	routes, err := c.aiRoutesByIndex(aiRouteIndexProvider, name)
	if err != nil {
		return
	}
	for _, route := range routes {
		for _, parentRef := range route.Spec.ParentRefs {
			c.enqueueGateway(parentRef)
		}
	}

	models, err := c.aiModelsByIndex(aiModelIndexProvider, name)
	if err != nil {
		return
	}
	for _, model := range models {
		c.enqueueAIModel(model.Name)
	}
}

func (c *Controller) enqueueAIModelObject(obj any) {
	model, ok := objectAs[*resource.AIModel](obj)
	if !ok {
		return
	}

	c.enqueueAIModel(model.Name)
}

func (c *Controller) enqueueAIModel(name string) {
	routes, err := c.aiRoutesByIndex(aiRouteIndexModel, name)
	if err != nil {
		return
	}
	for _, route := range routes {
		for _, parentRef := range route.Spec.ParentRefs {
			c.enqueueGateway(parentRef)
		}
	}
}

func (c *Controller) enqueueAIPolicyObject(obj any) {
	policy, ok := objectAs[*resource.AIPolicy](obj)
	if !ok {
		return
	}

	routes, err := c.aiRoutesByIndex(aiRouteIndexPolicy, policy.Name)
	if err != nil {
		return
	}
	for _, route := range routes {
		for _, parentRef := range route.Spec.ParentRefs {
			c.enqueueGateway(parentRef)
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

func (c *Controller) enqueuePolicyBindingObject(obj any) {
	binding, ok := objectAs[*resource.PolicyBinding](obj)
	if !ok {
		return
	}

	c.enqueuePolicyBinding(binding)
}

func (c *Controller) enqueuePolicyBinding(binding *resource.PolicyBinding) {
	// PolicyBinding 只影响 Gateway、Route、Upstream 这条通用 API 网关链路
	// AI 专属策略先由 AIPolicy 通过 AIRoute 引用，避免两套策略语义混在一起
	c.enqueueGatewayByTarget(binding.Spec.TargetRef.Kind, binding.Spec.TargetRef.Name)
}

func (c *Controller) enqueuePluginObject(obj any) {
	plugin, ok := objectAs[*resource.Plugin](obj)
	if !ok {
		return
	}

	// Plugin 本身不直接知道会影响哪些 Gateway
	// 需要先找到引用它的 PluginBinding，再根据 binding 的目标资源反向入队 Gateway
	bindings, err := c.pluginBindingsByIndex(pluginBindingIndexPlugin, plugin.Name)
	if err != nil {
		return
	}
	for _, binding := range bindings {
		c.enqueuePluginBinding(binding)
	}
}

func (c *Controller) enqueuePluginBindingObject(obj any) {
	binding, ok := objectAs[*resource.PluginBinding](obj)
	if !ok {
		return
	}

	c.enqueuePluginBinding(binding)
}

func (c *Controller) enqueuePluginBinding(binding *resource.PluginBinding) {
	// PluginBinding 的目标可以是 Gateway、普通 Route/Upstream，也可以是 AI 资源
	// controller 这里只负责找到关联 Gateway，真正的插件配置会在 bundleForGateway 中进入编译流水线
	target := binding.Spec.TargetRef
	switch target.Kind {
	case resource.KindGateway:
		c.enqueueGatewayByTarget(target.Kind, target.Name)
	case resource.KindRoute:
		c.enqueueGatewayByTarget(target.Kind, target.Name)
	case resource.KindUpstream:
		c.enqueueGatewayByTarget(target.Kind, target.Name)
	case resource.KindAIRoute:
		route, err := c.aiRouteLister.Get(target.Name)
		if err != nil {
			return
		}
		for _, parentRef := range route.Spec.ParentRefs {
			c.enqueueGateway(parentRef)
		}
	case resource.KindAIProvider:
		c.enqueueAIProvider(target.Name)
	case resource.KindAIModel:
		c.enqueueAIModel(target.Name)
	}
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
			c.enqueueGateway(parentRef)
		}
	case resource.KindUpstream:
		routes, err := c.routesByIndex(routeIndexUpstreamRef, name)
		if err != nil {
			return
		}
		for _, route := range routes {
			for _, parentRef := range route.Spec.ParentRefs {
				c.enqueueGateway(parentRef)
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

func (c *Controller) aiRoutesByIndex(index routeIndexName, value string) ([]*resource.AIRoute, error) {
	items, err := c.aiRouteIndexer.ByIndex(string(index), value)
	if err != nil {
		return nil, err
	}

	routes := make([]*resource.AIRoute, 0, len(items))
	for _, item := range items {
		route, ok := item.(*resource.AIRoute)
		if !ok {
			continue
		}
		routes = append(routes, route)
	}
	return routes, nil
}

func (c *Controller) aiModelsByIndex(index routeIndexName, value string) ([]*resource.AIModel, error) {
	items, err := c.aiModelIndexer.ByIndex(string(index), value)
	if err != nil {
		return nil, err
	}

	models := make([]*resource.AIModel, 0, len(items))
	for _, item := range items {
		model, ok := item.(*resource.AIModel)
		if !ok {
			continue
		}
		models = append(models, model)
	}
	return models, nil
}

func (c *Controller) pluginBindingsByIndex(index routeIndexName, value string) ([]*resource.PluginBinding, error) {
	items, err := c.pluginBindingIndexer.ByIndex(string(index), value)
	if err != nil {
		return nil, err
	}

	bindings := make([]*resource.PluginBinding, 0, len(items))
	for _, item := range items {
		binding, ok := item.(*resource.PluginBinding)
		if !ok {
			continue
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
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
	return uniqueStrings(route.Spec.ParentRefs), nil
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

func aiRouteParentRefIndex(obj any) ([]string, error) {
	route, ok := obj.(*resource.AIRoute)
	if !ok {
		return nil, nil
	}
	return uniqueStrings(route.Spec.ParentRefs), nil
}

func aiRouteProviderRefIndex(obj any) ([]string, error) {
	route, ok := obj.(*resource.AIRoute)
	if !ok {
		return nil, nil
	}

	providerNames := make([]string, 0, len(route.Spec.ProviderRefs))
	for _, providerRef := range route.Spec.ProviderRefs {
		providerNames = append(providerNames, providerRef.Name)
	}
	return uniqueStrings(providerNames), nil
}

func aiRouteModelRefIndex(obj any) ([]string, error) {
	route, ok := obj.(*resource.AIRoute)
	if !ok {
		return nil, nil
	}

	modelNames := make([]string, 0, len(route.Spec.Models))
	for _, modelRef := range route.Spec.Models {
		modelNames = append(modelNames, modelRef.Name)
	}
	return uniqueStrings(modelNames), nil
}

func aiRoutePolicyRefIndex(obj any) ([]string, error) {
	route, ok := obj.(*resource.AIRoute)
	if !ok {
		return nil, nil
	}
	return uniqueStrings(route.Spec.PolicyRefs), nil
}

func aiModelProviderRefIndex(obj any) ([]string, error) {
	model, ok := obj.(*resource.AIModel)
	if !ok {
		return nil, nil
	}
	return uniqueStrings([]string{model.Spec.ProviderRef}), nil
}

func pluginBindingTargetRefIndex(obj any) ([]string, error) {
	binding, ok := obj.(*resource.PluginBinding)
	if !ok {
		return nil, nil
	}
	return uniqueStrings([]string{targetIndexValue(binding.Spec.TargetRef.Kind, binding.Spec.TargetRef.Name)}), nil
}

func pluginBindingPluginRefIndex(obj any) ([]string, error) {
	binding, ok := obj.(*resource.PluginBinding)
	if !ok {
		return nil, nil
	}

	pluginNames := make([]string, 0, len(binding.Spec.Plugins))
	for _, pluginRef := range binding.Spec.Plugins {
		pluginNames = append(pluginNames, pluginRef.Name)
	}
	return uniqueStrings(pluginNames), nil
}

func policyBindingTargetRefIndex(obj any) ([]string, error) {
	binding, ok := obj.(*resource.PolicyBinding)
	if !ok {
		return nil, nil
	}
	return uniqueStrings([]string{targetIndexValue(binding.Spec.TargetRef.Kind, binding.Spec.TargetRef.Name)}), nil
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
