package controller

import (
	"context"
	"maps"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func (c *Controller) reconcileGateway(gatewayName string) error {
	bundle, found, err := c.bundleForGateway(gatewayName)
	if err != nil {
		return err
	}
	if !found {
		if err := c.deleteRuntimeSnapshot(context.Background(), c.target, gatewayName); err != nil {
			return err
		}
		c.logger.Info("runtime snapshot deleted",
			"target", c.target,
			"gateway", gatewayName,
			"reason", "gateway_not_found",
		)
		return nil
	}
	if !bundle.Gateways[0].Spec.Enabled {
		if err := c.deleteRuntimeSnapshot(context.Background(), c.target, gatewayName); err != nil {
			return err
		}
		c.logger.Info("runtime snapshot deleted",
			"target", c.target,
			"gateway", gatewayName,
			"reason", "gateway_disabled",
		)
		return nil
	}

	snapshot, err := c.pipeline.BuildGatewaySnapshotForTarget(bundle, gatewayName, c.target)
	if err != nil {
		return err
	}
	if err := c.upsertRuntimeSnapshot(context.Background(), snapshot); err != nil {
		return err
	}
	c.logger.Info("gateway reconciled",
		"target", c.target,
		"gateway", snapshot.Gateway,
		"routes", len(bundle.Routes),
		"ai_routes", len(bundle.AIRoutes),
		"upstreams", len(bundle.Upstreams),
		"ai_providers", len(bundle.AIProviders),
		"snapshot", snapshot.Version,
	)
	return nil
}

func (c *Controller) bundleForGateway(gatewayName string) (resource.Bundle, bool, error) {
	gateway, err := c.gatewayLister.Get(gatewayName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return resource.Bundle{}, false, nil
		}
		return resource.Bundle{}, false, err
	}

	routes, err := c.routesByIndex(routeIndexParentRef, gatewayName)
	if err != nil {
		return resource.Bundle{}, false, err
	}
	aiRoutes, err := c.aiRoutesByIndex(aiRouteIndexParentRef, gatewayName)
	if err != nil {
		return resource.Bundle{}, false, err
	}

	bundle := resource.Bundle{
		Gateways: []resource.Gateway{*gateway},
		Routes:   make([]resource.Route, 0, len(routes)),
		AIRoutes: make([]resource.AIRoute, 0, len(aiRoutes)),
	}

	usedUpstreams := c.appendRoutes(&bundle, routes)
	if err := c.appendUpstreams(&bundle, usedUpstreams); err != nil {
		return resource.Bundle{}, false, err
	}
	if err := c.appendAIResources(&bundle, aiRoutes); err != nil {
		return resource.Bundle{}, false, err
	}
	if err := c.appendBindingResources(&bundle, gateway.Name); err != nil {
		return resource.Bundle{}, false, err
	}
	return bundle, true, nil
}

func (c *Controller) appendRoutes(bundle *resource.Bundle, routes []*resource.Route) map[string]bool {
	usedUpstreams := map[string]bool{}
	for _, route := range routes {
		if !route.Spec.Enabled {
			continue
		}
		bundle.Routes = append(bundle.Routes, *route)
		for _, rule := range route.Spec.Rules {
			for _, upstreamRef := range rule.UpstreamRefs {
				usedUpstreams[upstreamRef.Name] = true
			}
		}
	}
	return usedUpstreams
}

func (c *Controller) appendUpstreams(bundle *resource.Bundle, names map[string]bool) error {
	for _, upstreamName := range slices.Sorted(maps.Keys(names)) {
		upstream, err := c.upstreamLister.Get(upstreamName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.Upstreams = append(bundle.Upstreams, *upstream)
	}
	return nil
}

func (c *Controller) appendAIResources(bundle *resource.Bundle, routes []*resource.AIRoute) error {
	usedAIModels := map[string]bool{}
	usedAIProviders := map[string]bool{}
	usedAIPolicies := map[string]bool{}
	for _, route := range routes {
		// AIRoute 只声明 AI 请求入口，真正的供应商、模型映射和策略仍然来自一等资源
		// 这里把引用资源补进 bundle，后续 compiler/xDS translator 才能生成 provider cluster 和 Wasm _rules_
		bundle.AIRoutes = append(bundle.AIRoutes, *route)
		for _, modelRef := range route.Spec.Models {
			usedAIModels[modelRef.Name] = true
		}
		for _, providerRef := range route.Spec.ProviderRefs {
			usedAIProviders[providerRef.Name] = true
		}
		for _, policyRef := range route.Spec.PolicyRefs {
			usedAIPolicies[policyRef] = true
		}
	}

	bundle.AIModels = make([]resource.AIModel, 0, len(usedAIModels))
	for _, modelName := range slices.Sorted(maps.Keys(usedAIModels)) {
		model, err := c.aiModelLister.Get(modelName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.AIModels = append(bundle.AIModels, *model)
		if model.Spec.ProviderRef != "" {
			usedAIProviders[model.Spec.ProviderRef] = true
		}
	}
	bundle.AIProviders = make([]resource.AIProvider, 0, len(usedAIProviders))
	for _, providerName := range slices.Sorted(maps.Keys(usedAIProviders)) {
		provider, err := c.aiProviderLister.Get(providerName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.AIProviders = append(bundle.AIProviders, *provider)
	}
	bundle.AIPolicies = make([]resource.AIPolicy, 0, len(usedAIPolicies))
	for _, policyName := range slices.Sorted(maps.Keys(usedAIPolicies)) {
		policy, err := c.aiPolicyLister.Get(policyName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.AIPolicies = append(bundle.AIPolicies, *policy)
	}
	return nil
}

func (c *Controller) appendBindingResources(bundle *resource.Bundle, gatewayName string) error {
	usedPluginBindings := map[string]bool{}
	usedPlugins := map[string]bool{}
	usedAuthPolicies := map[string]bool{}
	usedRateLimitPolicies := map[string]bool{}
	if err := c.appendPluginBindingsForTarget(bundle, usedPluginBindings, usedPlugins, resource.KindGateway, gatewayName); err != nil {
		return err
	}
	if err := c.appendPolicyBindingsForTarget(bundle, usedAuthPolicies, usedRateLimitPolicies, resource.KindGateway, gatewayName); err != nil {
		return err
	}
	for _, route := range bundle.Routes {
		if err := c.appendPluginBindingsForTarget(bundle, usedPluginBindings, usedPlugins, resource.KindRoute, route.Name); err != nil {
			return err
		}
		if err := c.appendPolicyBindingsForTarget(bundle, usedAuthPolicies, usedRateLimitPolicies, resource.KindRoute, route.Name); err != nil {
			return err
		}
	}
	for _, upstream := range bundle.Upstreams {
		if err := c.appendPluginBindingsForTarget(bundle, usedPluginBindings, usedPlugins, resource.KindUpstream, upstream.Name); err != nil {
			return err
		}
		if err := c.appendPolicyBindingsForTarget(bundle, usedAuthPolicies, usedRateLimitPolicies, resource.KindUpstream, upstream.Name); err != nil {
			return err
		}
	}
	for _, route := range bundle.AIRoutes {
		if err := c.appendPluginBindingsForTarget(bundle, usedPluginBindings, usedPlugins, resource.KindAIRoute, route.Name); err != nil {
			return err
		}
	}
	for _, provider := range bundle.AIProviders {
		if err := c.appendPluginBindingsForTarget(bundle, usedPluginBindings, usedPlugins, resource.KindAIProvider, provider.Name); err != nil {
			return err
		}
	}
	for _, model := range bundle.AIModels {
		if err := c.appendPluginBindingsForTarget(bundle, usedPluginBindings, usedPlugins, resource.KindAIModel, model.Name); err != nil {
			return err
		}
	}

	if err := c.appendPlugins(bundle, usedPlugins); err != nil {
		return err
	}
	if err := c.appendAuthPolicies(bundle, usedAuthPolicies); err != nil {
		return err
	}
	return c.appendRateLimitPolicies(bundle, usedRateLimitPolicies)
}

func (c *Controller) appendPolicyBindingsForTarget(bundle *resource.Bundle, usedAuthPolicies, usedRateLimitPolicies map[string]bool, kind resource.Kind, name string) error {
	// PolicyBinding 通过 Gateway/Route/Upstream 生效
	// 这里补进 bundle 后，compiler 会按绑定关系生成 AuthPolicy 和 RateLimitPolicy 的逻辑配置
	bindings, err := c.policyBindingsByIndex(policyBindingIndexTargetRef, targetIndexValue(kind, name))
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		bundle.PolicyBindings = append(bundle.PolicyBindings, *binding)
		for _, policyRef := range binding.Spec.Policies {
			switch policyRef.Kind {
			case resource.KindAuthPolicy:
				usedAuthPolicies[policyRef.Name] = true
			case resource.KindRateLimitPolicy:
				usedRateLimitPolicies[policyRef.Name] = true
			}
		}
	}
	return nil
}

func (c *Controller) appendPluginBindingsForTarget(bundle *resource.Bundle, usedPluginBindings, usedPlugins map[string]bool, kind resource.Kind, name string) error {
	// PluginBinding 通过目标资源间接作用到 Gateway
	// 例如 AIRoute 绑定 ai-proxy 后，xDS translator 会把它合并成 Gateway 级 Wasm filter 配置
	bindings, err := c.pluginBindingsByIndex(pluginBindingIndexTargetRef, targetIndexValue(kind, name))
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if usedPluginBindings[binding.Name] {
			continue
		}
		usedPluginBindings[binding.Name] = true
		bundle.PluginBindings = append(bundle.PluginBindings, *binding)
		for _, pluginRef := range binding.Spec.Plugins {
			usedPlugins[pluginRef.Name] = true
		}
	}
	return nil
}

func (c *Controller) appendPlugins(bundle *resource.Bundle, names map[string]bool) error {
	bundle.Plugins = make([]resource.Plugin, 0, len(names))
	for _, pluginName := range slices.Sorted(maps.Keys(names)) {
		plugin, err := c.pluginLister.Get(pluginName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.Plugins = append(bundle.Plugins, *plugin)
	}
	return nil
}

func (c *Controller) appendAuthPolicies(bundle *resource.Bundle, names map[string]bool) error {
	bundle.AuthPolicies = make([]resource.AuthPolicy, 0, len(names))
	for _, policyName := range slices.Sorted(maps.Keys(names)) {
		policy, err := c.authPolicyLister.Get(policyName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.AuthPolicies = append(bundle.AuthPolicies, *policy)
	}
	return nil
}

func (c *Controller) appendRateLimitPolicies(bundle *resource.Bundle, names map[string]bool) error {
	bundle.RateLimitPolicies = make([]resource.RateLimitPolicy, 0, len(names))
	for _, policyName := range slices.Sorted(maps.Keys(names)) {
		policy, err := c.rateLimitLister.Get(policyName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.RateLimitPolicies = append(bundle.RateLimitPolicies, *policy)
	}
	return nil
}
