package controller

import (
	"context"
	"fmt"
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
		fmt.Fprintf(c.stdout, "deleted target=%s gateway=%s reason=gateway-not-found\n", c.target, gatewayName)
		return nil
	}

	snapshot, err := c.pipeline.BuildGatewaySnapshotForTarget(bundle, gatewayName, c.target)
	if err != nil {
		return err
	}
	if err := c.upsertRuntimeSnapshot(context.Background(), snapshot); err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "reconciled target=%s gateway=%s routes=%d aiRoutes=%d upstreams=%d aiProviders=%d snapshot=%s\n",
		c.target,
		snapshot.Gateway,
		len(bundle.Routes),
		len(bundle.AIRoutes),
		len(bundle.Upstreams),
		len(bundle.AIProviders),
		snapshot.Version,
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

	usedUpstreams := map[string]bool{}
	for _, route := range routes {
		bundle.Routes = append(bundle.Routes, *route)
		for _, rule := range route.Spec.Rules {
			for _, upstreamRef := range rule.UpstreamRefs {
				usedUpstreams[upstreamRef.Name] = true
			}
		}
	}
	for _, upstreamName := range slices.Sorted(maps.Keys(usedUpstreams)) {
		upstream, err := c.upstreamLister.Get(upstreamName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return resource.Bundle{}, false, err
		}
		bundle.Upstreams = append(bundle.Upstreams, *upstream)
	}

	usedAIModels := map[string]bool{}
	usedAIProviders := map[string]bool{}
	usedAIPolicies := map[string]bool{}
	for _, route := range aiRoutes {
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
			return resource.Bundle{}, false, err
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
			return resource.Bundle{}, false, err
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
			return resource.Bundle{}, false, err
		}
		bundle.AIPolicies = append(bundle.AIPolicies, *policy)
	}

	usedPluginBindings := map[string]bool{}
	usedPlugins := map[string]bool{}
	addPluginBindings := func(kind resource.Kind, name string) error {
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
	if err := addPluginBindings(resource.KindGateway, gateway.Name); err != nil {
		return resource.Bundle{}, false, err
	}
	for _, route := range bundle.Routes {
		if err := addPluginBindings(resource.KindRoute, route.Name); err != nil {
			return resource.Bundle{}, false, err
		}
	}
	for _, upstream := range bundle.Upstreams {
		if err := addPluginBindings(resource.KindUpstream, upstream.Name); err != nil {
			return resource.Bundle{}, false, err
		}
	}
	for _, route := range bundle.AIRoutes {
		if err := addPluginBindings(resource.KindAIRoute, route.Name); err != nil {
			return resource.Bundle{}, false, err
		}
	}
	for _, provider := range bundle.AIProviders {
		if err := addPluginBindings(resource.KindAIProvider, provider.Name); err != nil {
			return resource.Bundle{}, false, err
		}
	}
	for _, model := range bundle.AIModels {
		if err := addPluginBindings(resource.KindAIModel, model.Name); err != nil {
			return resource.Bundle{}, false, err
		}
	}

	bundle.Plugins = make([]resource.Plugin, 0, len(usedPlugins))
	for _, pluginName := range slices.Sorted(maps.Keys(usedPlugins)) {
		plugin, err := c.pluginLister.Get(pluginName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return resource.Bundle{}, false, err
		}
		bundle.Plugins = append(bundle.Plugins, *plugin)
	}
	return bundle, true, nil
}
