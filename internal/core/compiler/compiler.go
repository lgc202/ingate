// Package compiler 将声明式资源编译成运行时无关的 IR
package compiler

import (
	"fmt"
	"slices"

	"github.com/lgc202/ingate/internal/core/ir"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Compiler 负责把声明式资源编译成逻辑网关模型
type Compiler struct{}

type gatewayCompiler struct {
	bundle      resource.Bundle
	gatewayName string
	gateway     resource.Gateway

	gatewaysByName          map[string]resource.Gateway
	routesByName            map[string]bool
	aiRoutesByName          map[string]bool
	upstreamsByName         map[string]resource.Upstream
	aiProvidersByName       map[string]resource.AIProvider
	aiModelsByName          map[string]resource.AIModel
	aiPoliciesByName        map[string]resource.AIPolicy
	pluginsByName           map[string]resource.Plugin
	authPoliciesByName      map[string]resource.AuthPolicy
	rateLimitPoliciesByName map[string]resource.RateLimitPolicy
	redisStoresByName       map[string]resource.RedisStore
	routeRulesByRoute       map[string]map[string]bool
	policyBindingsByName    map[string]bool
	pluginBindingsByName    map[string]bool
}

type attachedAIRoutes struct {
	routes        []ir.LogicalAIRoute
	modelOrder    []string
	providerOrder []string
	policyOrder   []string
}

// CompileGateway 从内存资源集合中编译指定 Gateway
func (Compiler) CompileGateway(bundle resource.Bundle, gatewayName string) (ir.LogicalGateway, error) {
	c := gatewayCompiler{
		bundle:                  bundle,
		gatewayName:             gatewayName,
		gatewaysByName:          make(map[string]resource.Gateway, len(bundle.Gateways)),
		routesByName:            make(map[string]bool, len(bundle.Routes)),
		aiRoutesByName:          make(map[string]bool, len(bundle.AIRoutes)),
		upstreamsByName:         make(map[string]resource.Upstream, len(bundle.Upstreams)),
		aiProvidersByName:       make(map[string]resource.AIProvider, len(bundle.AIProviders)),
		aiModelsByName:          make(map[string]resource.AIModel, len(bundle.AIModels)),
		aiPoliciesByName:        make(map[string]resource.AIPolicy, len(bundle.AIPolicies)),
		pluginsByName:           make(map[string]resource.Plugin, len(bundle.Plugins)),
		authPoliciesByName:      make(map[string]resource.AuthPolicy, len(bundle.AuthPolicies)),
		rateLimitPoliciesByName: make(map[string]resource.RateLimitPolicy, len(bundle.RateLimitPolicies)),
		redisStoresByName:       make(map[string]resource.RedisStore, len(bundle.RedisStores)),
		routeRulesByRoute:       make(map[string]map[string]bool, len(bundle.Routes)),
		policyBindingsByName:    make(map[string]bool, len(bundle.PolicyBindings)),
		pluginBindingsByName:    make(map[string]bool, len(bundle.PluginBindings)),
	}

	return c.compile()
}

func (c *gatewayCompiler) compile() (ir.LogicalGateway, error) {
	if err := c.indexGateways(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexRoutes(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexAIRoutes(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexUpstreams(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexAIProviders(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexAIModels(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexAIPolicies(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexPlugins(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexAuthPolicies(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexRedisStores(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexRateLimitPolicies(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexPolicyBindings(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexPluginBindings(); err != nil {
		return ir.LogicalGateway{}, err
	}

	routes, upstreamOrder, err := c.buildAttachedRoutes()
	if err != nil {
		return ir.LogicalGateway{}, err
	}
	aiRoutes, err := c.buildAttachedAIRoutes()
	if err != nil {
		return ir.LogicalGateway{}, err
	}
	policyBindings := c.buildPolicyBindings(routes, upstreamOrder)
	pluginBindings := c.buildPluginBindings(routes, upstreamOrder, aiRoutes)
	rateLimitPolicies, redisStoreNames := c.buildRateLimitPolicies(policyBindings)

	return ir.LogicalGateway{
		Name:              c.gateway.Name,
		Listeners:         c.buildListeners(),
		Routes:            routes,
		AIRoutes:          aiRoutes.routes,
		Upstreams:         c.buildUsedUpstreams(upstreamOrder),
		AIProviders:       c.buildUsedAIProviders(aiRoutes.providerOrder),
		AIModels:          c.buildUsedAIModels(aiRoutes.modelOrder),
		AIPolicies:        c.buildUsedAIPolicies(aiRoutes.policyOrder),
		Plugins:           c.buildPlugins(pluginBindings),
		AuthPolicies:      c.buildAuthPolicies(policyBindings),
		RateLimitPolicies: rateLimitPolicies,
		RedisStores:       c.buildRedisStores(redisStoreNames),
		PolicyBindings:    policyBindings,
		PluginBindings:    pluginBindings,
	}, nil
}

func (c *gatewayCompiler) indexGateways() error {
	for _, gateway := range c.bundle.Gateways {
		if _, ok := c.gatewaysByName[gateway.Name]; ok {
			return fmt.Errorf("duplicate gateway %q", gateway.Name)
		}
		c.gatewaysByName[gateway.Name] = gateway
	}

	gateway, ok := c.gatewaysByName[c.gatewayName]
	if !ok {
		return fmt.Errorf("gateway %q not found", c.gatewayName)
	}
	if !gateway.Spec.Enabled {
		return fmt.Errorf("gateway %q is disabled", c.gatewayName)
	}
	c.gateway = gateway

	return nil
}

func (c *gatewayCompiler) indexRoutes() error {
	for _, route := range c.bundle.Routes {
		if c.routesByName[route.Name] {
			return fmt.Errorf("duplicate route %q", route.Name)
		}
		c.routesByName[route.Name] = true
		rules := make(map[string]bool, len(route.Spec.Rules))
		for _, rule := range route.Spec.Rules {
			if rule.Name == "" {
				continue
			}
			if rules[rule.Name] {
				return fmt.Errorf("route %q has duplicate rule %q", route.Name, rule.Name)
			}
			rules[rule.Name] = true
		}
		c.routeRulesByRoute[route.Name] = rules
	}

	return nil
}

func (c *gatewayCompiler) indexAIRoutes() error {
	for _, route := range c.bundle.AIRoutes {
		if c.aiRoutesByName[route.Name] {
			return fmt.Errorf("duplicate ai route %q", route.Name)
		}
		c.aiRoutesByName[route.Name] = true
	}

	return nil
}

func (c *gatewayCompiler) indexUpstreams() error {
	for _, upstream := range c.bundle.Upstreams {
		if _, ok := c.upstreamsByName[upstream.Name]; ok {
			return fmt.Errorf("duplicate upstream %q", upstream.Name)
		}
		c.upstreamsByName[upstream.Name] = upstream
	}

	return nil
}

func (c *gatewayCompiler) indexAIProviders() error {
	for _, provider := range c.bundle.AIProviders {
		if _, ok := c.aiProvidersByName[provider.Name]; ok {
			return fmt.Errorf("duplicate ai provider %q", provider.Name)
		}
		c.aiProvidersByName[provider.Name] = provider
	}

	return nil
}

func (c *gatewayCompiler) indexAIModels() error {
	for _, model := range c.bundle.AIModels {
		if _, ok := c.aiModelsByName[model.Name]; ok {
			return fmt.Errorf("duplicate ai model %q", model.Name)
		}
		c.aiModelsByName[model.Name] = model
	}

	return nil
}

func (c *gatewayCompiler) indexAIPolicies() error {
	for _, policy := range c.bundle.AIPolicies {
		if _, ok := c.aiPoliciesByName[policy.Name]; ok {
			return fmt.Errorf("duplicate ai policy %q", policy.Name)
		}
		c.aiPoliciesByName[policy.Name] = policy
	}

	return nil
}

func (c *gatewayCompiler) indexPlugins() error {
	for _, plugin := range c.bundle.Plugins {
		if _, ok := c.pluginsByName[plugin.Name]; ok {
			return fmt.Errorf("duplicate plugin %q", plugin.Name)
		}
		c.pluginsByName[plugin.Name] = plugin
	}

	return nil
}

func (c *gatewayCompiler) indexAuthPolicies() error {
	for _, policy := range c.bundle.AuthPolicies {
		if _, ok := c.authPoliciesByName[policy.Name]; ok {
			return fmt.Errorf("duplicate auth policy %q", policy.Name)
		}
		c.authPoliciesByName[policy.Name] = policy
	}

	return nil
}

func (c *gatewayCompiler) indexRateLimitPolicies() error {
	for _, policy := range c.bundle.RateLimitPolicies {
		if _, ok := c.rateLimitPoliciesByName[policy.Name]; ok {
			return fmt.Errorf("duplicate rate limit policy %q", policy.Name)
		}
		if policy.Spec.Enabled && policy.Spec.Mode == resource.RateLimitModeGlobal {
			if policy.Spec.Global == nil {
				return fmt.Errorf("rate limit policy %q requires global config", policy.Name)
			}
			if _, ok := c.redisStoresByName[policy.Spec.Global.RedisRef]; !ok {
				return fmt.Errorf("rate limit policy %q references redis store %q", policy.Name, policy.Spec.Global.RedisRef)
			}
		}
		c.rateLimitPoliciesByName[policy.Name] = policy
	}

	return nil
}

func (c *gatewayCompiler) indexRedisStores() error {
	for _, store := range c.bundle.RedisStores {
		if _, ok := c.redisStoresByName[store.Name]; ok {
			return fmt.Errorf("duplicate redis store %q", store.Name)
		}
		c.redisStoresByName[store.Name] = store
	}

	return nil
}

func (c *gatewayCompiler) indexPolicyBindings() error {
	for _, binding := range c.bundle.PolicyBindings {
		if c.policyBindingsByName[binding.Name] {
			return fmt.Errorf("duplicate policy binding %q", binding.Name)
		}
		c.policyBindingsByName[binding.Name] = true
		if !binding.Spec.Enabled {
			continue
		}

		target := binding.Spec.TargetRef
		switch target.Kind {
		case resource.KindGateway:
			if _, ok := c.gatewaysByName[target.Name]; !ok {
				return fmt.Errorf("policy binding %q references gateway %q", binding.Name, target.Name)
			}
		case resource.KindRoute:
			if !c.routesByName[target.Name] {
				return fmt.Errorf("policy binding %q references route %q", binding.Name, target.Name)
			}
			if target.RuleName != "" && !c.routeRulesByRoute[target.Name][target.RuleName] {
				return fmt.Errorf("policy binding %q references route %q rule %q", binding.Name, target.Name, target.RuleName)
			}
		case resource.KindUpstream:
			if _, ok := c.upstreamsByName[target.Name]; !ok {
				return fmt.Errorf("policy binding %q references upstream %q", binding.Name, target.Name)
			}
		default:
			return fmt.Errorf("policy binding %q references unsupported kind %q", binding.Name, target.Kind)
		}

		for _, policy := range binding.Spec.Policies {
			switch policy.Kind {
			case resource.KindAuthPolicy:
				if _, ok := c.authPoliciesByName[policy.Name]; !ok {
					return fmt.Errorf("policy binding %q references auth policy %q", binding.Name, policy.Name)
				}
			case resource.KindRateLimitPolicy:
				if _, ok := c.rateLimitPoliciesByName[policy.Name]; !ok {
					return fmt.Errorf("policy binding %q references rate limit policy %q", binding.Name, policy.Name)
				}
			default:
				return fmt.Errorf("policy binding %q references unsupported policy kind %q", binding.Name, policy.Kind)
			}
		}
	}

	return nil
}

func (c *gatewayCompiler) indexPluginBindings() error {
	for _, binding := range c.bundle.PluginBindings {
		if c.pluginBindingsByName[binding.Name] {
			return fmt.Errorf("duplicate plugin binding %q", binding.Name)
		}
		c.pluginBindingsByName[binding.Name] = true

		target := binding.Spec.TargetRef
		switch target.Kind {
		case resource.KindGateway:
			if _, ok := c.gatewaysByName[target.Name]; !ok {
				return fmt.Errorf("plugin binding %q references gateway %q", binding.Name, target.Name)
			}
		case resource.KindRoute:
			if !c.routesByName[target.Name] {
				return fmt.Errorf("plugin binding %q references route %q", binding.Name, target.Name)
			}
		case resource.KindUpstream:
			if _, ok := c.upstreamsByName[target.Name]; !ok {
				return fmt.Errorf("plugin binding %q references upstream %q", binding.Name, target.Name)
			}
		case resource.KindAIRoute:
			if !c.aiRoutesByName[target.Name] {
				return fmt.Errorf("plugin binding %q references ai route %q", binding.Name, target.Name)
			}
		case resource.KindAIProvider:
			if _, ok := c.aiProvidersByName[target.Name]; !ok {
				return fmt.Errorf("plugin binding %q references ai provider %q", binding.Name, target.Name)
			}
		case resource.KindAIModel:
			if _, ok := c.aiModelsByName[target.Name]; !ok {
				return fmt.Errorf("plugin binding %q references ai model %q", binding.Name, target.Name)
			}
		default:
			return fmt.Errorf("plugin binding %q references unsupported kind %q", binding.Name, target.Kind)
		}

		for _, plugin := range binding.Spec.Plugins {
			if _, ok := c.pluginsByName[plugin.Name]; !ok {
				return fmt.Errorf("plugin binding %q references plugin %q", binding.Name, plugin.Name)
			}
		}
	}

	return nil
}

func (c *gatewayCompiler) buildListeners() []ir.LogicalListener {
	listeners := make([]ir.LogicalListener, 0, len(c.gateway.Spec.Listeners))
	for _, listener := range c.gateway.Spec.Listeners {
		listeners = append(listeners, ir.LogicalListener{
			Name:     listener.Name,
			Protocol: string(listener.Protocol),
			Port:     listener.Port,
			Hostname: c.listenerHostname(listener.Name),
		})
	}
	return listeners
}

func (c *gatewayCompiler) listenerHostname(listenerName string) string {
	hostname := ""
	for _, binding := range c.gateway.Spec.HostBindings {
		if !slices.Contains(binding.ListenerRefs, listenerName) {
			continue
		}
		if binding.Hostname == "" {
			return ""
		}
		if hostname != "" {
			return ""
		}
		hostname = binding.Hostname
	}
	return hostname
}

func (c *gatewayCompiler) buildAttachedRoutes() ([]ir.LogicalRoute, []string, error) {
	routes := make([]ir.LogicalRoute, 0, len(c.bundle.Routes))
	usedUpstreams := make(map[string]bool)
	var upstreamOrder []string

	for _, route := range c.bundle.Routes {
		if !route.Spec.Enabled || !routeHasParent(route, c.gatewayName) {
			continue
		}

		logicalRoute := ir.LogicalRoute{
			Name:      route.Name,
			Hostnames: slices.Clone(route.Spec.Hostnames),
			Rules:     make([]ir.LogicalRouteRule, 0, len(route.Spec.Rules)),
		}
		for _, rule := range route.Spec.Rules {
			logicalRule := ir.LogicalRouteRule{
				Name:       rule.Name,
				PathPrefix: rule.PathPrefix,
				Methods:    slices.Clone(rule.Methods),
				Upstreams:  make([]ir.LogicalUpstreamRef, 0, len(rule.UpstreamRefs)),
			}
			if rule.Timeout != nil {
				logicalRule.TimeoutMillis = rule.Timeout.RequestMillis
			}
			if rule.Retry != nil {
				logicalRule.Retry = ir.LogicalRetryPolicy{
					Attempts:            rule.Retry.Attempts,
					PerTryTimeoutMillis: rule.Retry.PerTryTimeoutMillis,
				}
			}
			if len(rule.Headers) > 0 {
				logicalRule.Headers = make([]ir.LogicalHeaderMatch, 0, len(rule.Headers))
				for _, header := range rule.Headers {
					logicalRule.Headers = append(logicalRule.Headers, ir.LogicalHeaderMatch{
						Name:  header.Name,
						Value: header.Value,
					})
				}
			}
			for _, upstreamRef := range rule.UpstreamRefs {
				if _, ok := c.upstreamsByName[upstreamRef.Name]; !ok {
					return nil, nil, fmt.Errorf("route %q references upstream %q", route.Name, upstreamRef.Name)
				}
				logicalRule.Upstreams = append(logicalRule.Upstreams, ir.LogicalUpstreamRef{
					Name:   upstreamRef.Name,
					Weight: upstreamRef.Weight,
				})
				if !usedUpstreams[upstreamRef.Name] {
					usedUpstreams[upstreamRef.Name] = true
					upstreamOrder = append(upstreamOrder, upstreamRef.Name)
				}
			}
			if err := c.applyRouteFilters(route.Name, rule.Filters, &logicalRule); err != nil {
				return nil, nil, err
			}
			logicalRoute.Rules = append(logicalRoute.Rules, logicalRule)
		}
		routes = append(routes, logicalRoute)
	}

	return routes, upstreamOrder, nil
}

func routeHasParent(route resource.Route, gatewayName string) bool {
	return slices.ContainsFunc(route.Spec.ParentRefs, func(parentRef resource.ParentRef) bool {
		return parentRef.Name == gatewayName
	})
}

func (c *gatewayCompiler) applyRouteFilters(routeName string, filters []resource.RouteFilter, logicalRule *ir.LogicalRouteRule) error {
	for _, filter := range filters {
		switch filter.Type {
		case resource.RouteFilterRequestHeaderModifier:
			if filter.RequestHeaderModifier == nil {
				return fmt.Errorf("route %q request header modifier is empty", routeName)
			}
			for _, header := range filter.RequestHeaderModifier.Set {
				logicalRule.RequestHeadersToAdd = append(logicalRule.RequestHeadersToAdd, ir.LogicalHeaderValue{
					Name:  header.Name,
					Value: header.Value,
				})
			}
			for _, header := range filter.RequestHeaderModifier.Add {
				logicalRule.RequestHeadersToAdd = append(logicalRule.RequestHeadersToAdd, ir.LogicalHeaderValue{
					Name:  header.Name,
					Value: header.Value,
				})
			}
			logicalRule.RequestHeadersToRemove = append(logicalRule.RequestHeadersToRemove, filter.RequestHeaderModifier.Remove...)
		default:
			return fmt.Errorf("route %q has unsupported route filter %q", routeName, filter.Type)
		}
	}
	return nil
}

func (c *gatewayCompiler) buildAttachedAIRoutes() (attachedAIRoutes, error) {
	result := attachedAIRoutes{
		routes: make([]ir.LogicalAIRoute, 0, len(c.bundle.AIRoutes)),
	}
	usedModels := make(map[string]bool)
	usedProviders := make(map[string]bool)
	usedPolicies := make(map[string]bool)

	for _, route := range c.bundle.AIRoutes {
		if !slices.Contains(route.Spec.ParentRefs, c.gatewayName) {
			continue
		}
		if len(route.Spec.Models) == 0 && len(route.Spec.ProviderRefs) == 0 {
			return attachedAIRoutes{}, fmt.Errorf("ai route %q has no ai models or ai providers", route.Name)
		}

		logicalRoute := ir.LogicalAIRoute{
			Name:       route.Name,
			Hostnames:  slices.Clone(route.Spec.Hostnames),
			Path:       route.Spec.Path,
			PathPrefix: route.Spec.PathPrefix,
			Model:      route.Spec.Model,
			Models:     make([]ir.LogicalAIModelRef, 0, len(route.Spec.Models)),
			Providers:  make([]ir.LogicalAIProviderRef, 0, len(route.Spec.ProviderRefs)),
			PolicyRefs: slices.Clone(route.Spec.PolicyRefs),
		}

		for _, modelRef := range route.Spec.Models {
			if modelRef.Weight <= 0 {
				return attachedAIRoutes{}, fmt.Errorf("ai route %q model %q has invalid weight %d", route.Name, modelRef.Name, modelRef.Weight)
			}
			model, ok := c.aiModelsByName[modelRef.Name]
			if !ok {
				return attachedAIRoutes{}, fmt.Errorf("ai route %q references ai model %q", route.Name, modelRef.Name)
			}
			if _, ok := c.aiProvidersByName[model.Spec.ProviderRef]; !ok {
				return attachedAIRoutes{}, fmt.Errorf("ai model %q references ai provider %q", model.Name, model.Spec.ProviderRef)
			}
			logicalRoute.Models = append(logicalRoute.Models, ir.LogicalAIModelRef{
				Name:   modelRef.Name,
				Weight: modelRef.Weight,
			})
			if !usedModels[modelRef.Name] {
				usedModels[modelRef.Name] = true
				result.modelOrder = append(result.modelOrder, modelRef.Name)
			}
			if !usedProviders[model.Spec.ProviderRef] {
				usedProviders[model.Spec.ProviderRef] = true
				result.providerOrder = append(result.providerOrder, model.Spec.ProviderRef)
			}
		}

		for _, providerRef := range route.Spec.ProviderRefs {
			if providerRef.Weight <= 0 {
				return attachedAIRoutes{}, fmt.Errorf("ai route %q provider %q has invalid weight %d", route.Name, providerRef.Name, providerRef.Weight)
			}
			if _, ok := c.aiProvidersByName[providerRef.Name]; !ok {
				return attachedAIRoutes{}, fmt.Errorf("ai route %q references ai provider %q", route.Name, providerRef.Name)
			}
			logicalRoute.Providers = append(logicalRoute.Providers, ir.LogicalAIProviderRef{
				Name:   providerRef.Name,
				Weight: providerRef.Weight,
			})
			if !usedProviders[providerRef.Name] {
				usedProviders[providerRef.Name] = true
				result.providerOrder = append(result.providerOrder, providerRef.Name)
			}
		}

		for _, policyRef := range route.Spec.PolicyRefs {
			if _, ok := c.aiPoliciesByName[policyRef]; !ok {
				return attachedAIRoutes{}, fmt.Errorf("ai route %q references ai policy %q", route.Name, policyRef)
			}
			if !usedPolicies[policyRef] {
				usedPolicies[policyRef] = true
				result.policyOrder = append(result.policyOrder, policyRef)
			}
		}
		result.routes = append(result.routes, logicalRoute)
	}

	return result, nil
}

func (c *gatewayCompiler) buildUsedUpstreams(upstreamOrder []string) []ir.LogicalUpstream {
	upstreams := make([]ir.LogicalUpstream, 0, len(upstreamOrder))
	for _, name := range upstreamOrder {
		upstream := c.upstreamsByName[name]
		logicalUpstream := ir.LogicalUpstream{
			Name:      upstream.Name,
			Endpoints: make([]ir.LogicalEndpoint, 0, len(upstream.Spec.Endpoints)),
		}
		for _, endpoint := range upstream.Spec.Endpoints {
			if !endpoint.Enabled {
				continue
			}
			logicalUpstream.Endpoints = append(logicalUpstream.Endpoints, ir.LogicalEndpoint{
				Address: endpoint.Address,
				Port:    endpoint.Port,
			})
		}
		upstreams = append(upstreams, logicalUpstream)
	}

	return upstreams
}

func (c *gatewayCompiler) buildUsedAIProviders(providerOrder []string) []ir.LogicalAIProvider {
	providers := make([]ir.LogicalAIProvider, 0, len(providerOrder))
	for _, name := range providerOrder {
		provider := c.aiProvidersByName[name]
		providers = append(providers, ir.LogicalAIProvider{
			Name:     provider.Name,
			Type:     provider.Spec.Type,
			Endpoint: provider.Spec.Endpoint,
			Models:   slices.Clone(provider.Spec.Models),
		})
	}

	return providers
}

func (c *gatewayCompiler) buildUsedAIModels(modelOrder []string) []ir.LogicalAIModel {
	models := make([]ir.LogicalAIModel, 0, len(modelOrder))
	for _, name := range modelOrder {
		model := c.aiModelsByName[name]
		models = append(models, ir.LogicalAIModel{
			Name:          model.Name,
			ProviderRef:   model.Spec.ProviderRef,
			ProviderModel: model.Spec.ProviderModel,
			Capabilities:  slices.Clone(model.Spec.Capabilities),
		})
	}

	return models
}

func (c *gatewayCompiler) buildUsedAIPolicies(policyOrder []string) []ir.LogicalAIPolicy {
	policies := make([]ir.LogicalAIPolicy, 0, len(policyOrder))
	for _, name := range policyOrder {
		policy := c.aiPoliciesByName[name]
		policies = append(policies, ir.LogicalAIPolicy{
			Name:            policy.Name,
			ExecutionTarget: policy.Spec.ExecutionTarget,
			TimeoutMillis:   policy.Spec.TimeoutMillis,
			RetryAttempts:   policy.Spec.Retry.Attempts,
			FallbackEnabled: policy.Spec.Fallback.Enabled,
			FallbackModels:  slices.Clone(policy.Spec.Fallback.Models),
			UsageEnabled:    policy.Spec.Usage.Enabled,
		})
	}

	return policies
}

func (c *gatewayCompiler) buildPlugins(bindings []ir.LogicalPluginBinding) []ir.LogicalPlugin {
	usedPlugins := make(map[string]bool)
	var pluginOrder []string

	for _, binding := range bindings {
		for _, plugin := range binding.Plugins {
			if usedPlugins[plugin.Name] {
				continue
			}
			usedPlugins[plugin.Name] = true
			pluginOrder = append(pluginOrder, plugin.Name)
		}
	}

	plugins := make([]ir.LogicalPlugin, 0, len(pluginOrder))
	for _, name := range pluginOrder {
		plugin := c.pluginsByName[name]
		plugins = append(plugins, ir.LogicalPlugin{
			Name:     plugin.Name,
			Runtime:  plugin.Spec.Runtime,
			Version:  plugin.Spec.Version,
			Endpoint: plugin.Spec.Endpoint,
			Image:    plugin.Spec.Image,
		})
	}

	return plugins
}

func (c *gatewayCompiler) buildAuthPolicies(bindings []ir.LogicalPolicyBinding) []ir.LogicalAuthPolicy {
	usedPolicies := make(map[string]bool)
	var policyOrder []string

	for _, binding := range bindings {
		for _, policy := range binding.Policies {
			if policy.Kind != resource.KindAuthPolicy || usedPolicies[policy.Name] {
				continue
			}
			usedPolicies[policy.Name] = true
			policyOrder = append(policyOrder, policy.Name)
		}
	}

	policies := make([]ir.LogicalAuthPolicy, 0, len(policyOrder))
	for _, name := range policyOrder {
		policy := c.authPoliciesByName[name]
		policies = append(policies, ir.LogicalAuthPolicy{
			Name: policy.Name,
			Type: policy.Spec.Type,
			APIKey: ir.LogicalAPIKeyAuth{
				Header: policy.Spec.APIKey.Header,
				Query:  policy.Spec.APIKey.Query,
			},
		})
	}

	return policies
}

func (c *gatewayCompiler) buildRateLimitPolicies(bindings []ir.LogicalPolicyBinding) ([]ir.LogicalRateLimitPolicy, map[string]bool) {
	usedPolicies := make(map[string]bool)
	usedRedisStores := make(map[string]bool)
	var policyOrder []string

	for _, binding := range bindings {
		for _, policy := range binding.Policies {
			if policy.Kind != resource.KindRateLimitPolicy || usedPolicies[policy.Name] {
				continue
			}
			rateLimitPolicy := c.rateLimitPoliciesByName[policy.Name]
			if !rateLimitPolicy.Spec.Enabled {
				continue
			}
			usedPolicies[policy.Name] = true
			policyOrder = append(policyOrder, policy.Name)
		}
	}

	policies := make([]ir.LogicalRateLimitPolicy, 0, len(policyOrder))
	for _, name := range policyOrder {
		policy := c.rateLimitPoliciesByName[name]
		logicalPolicy := ir.LogicalRateLimitPolicy{
			Name:          policy.Name,
			DisplayName:   policy.Spec.DisplayName,
			Mode:          policy.Spec.Mode,
			Rules:         make([]ir.LogicalRateLimitRule, 0, len(policy.Spec.Rules)),
			Response:      c.buildRateLimitResponse(policy.Spec.Response),
			FailurePolicy: policy.Spec.FailurePolicy,
		}
		for _, rule := range policy.Spec.Rules {
			logicalPolicy.Rules = append(logicalPolicy.Rules, ir.LogicalRateLimitRule{
				Name:      rule.Name,
				Key:       c.buildRateLimitKey(rule.Key),
				Limit:     ir.LogicalRateLimitQuota(rule.Limit),
				Algorithm: rule.Algorithm,
			})
		}
		if policy.Spec.Global != nil {
			logicalPolicy.Global = &ir.LogicalGlobalRateLimit{
				RedisRef:      policy.Spec.Global.RedisRef,
				Prefix:        policy.Spec.Global.Prefix,
				TimeoutMillis: policy.Spec.Global.TimeoutMillis,
			}
			usedRedisStores[policy.Spec.Global.RedisRef] = true
		}
		policies = append(policies, logicalPolicy)
	}

	return policies, usedRedisStores
}

func (c *gatewayCompiler) buildRateLimitKey(key resource.RateLimitKey) []ir.LogicalRateLimitKeyPart {
	parts := make([]ir.LogicalRateLimitKeyPart, 0, len(key.Parts))
	for _, part := range key.Parts {
		parts = append(parts, ir.LogicalRateLimitKeyPart{
			Type: part.Type,
			Name: part.Name,
		})
	}
	return parts
}

func (c *gatewayCompiler) buildRateLimitResponse(response resource.RateLimitResponse) ir.LogicalRateLimitResponse {
	return ir.LogicalRateLimitResponse{
		StatusCode:         response.StatusCode,
		Message:            response.Message,
		QuotaHeaderEnabled: response.QuotaHeaderEnabled,
	}
}

func (c *gatewayCompiler) buildRedisStores(names map[string]bool) []ir.LogicalRedisStore {
	if len(names) == 0 {
		return nil
	}

	var storeOrder []string
	for name := range names {
		storeOrder = append(storeOrder, name)
	}
	slices.Sort(storeOrder)

	stores := make([]ir.LogicalRedisStore, 0, len(names))
	for _, name := range storeOrder {
		store := c.redisStoresByName[name]
		stores = append(stores, ir.LogicalRedisStore{
			Name:                 store.Name,
			DisplayName:          store.Spec.DisplayName,
			Mode:                 store.Spec.Mode,
			Address:              store.Spec.Address,
			Addresses:            slices.Clone(store.Spec.Addresses),
			DB:                   store.Spec.DB,
			TLS:                  store.Spec.TLS,
			TLSServerName:        store.Spec.TLSServerName,
			Username:             store.Spec.Username,
			PasswordRef:          store.Spec.PasswordRef,
			ConnectTimeoutMillis: store.Spec.ConnectTimeoutMillis,
			CommandTimeoutMillis: store.Spec.CommandTimeoutMillis,
			PoolSize:             store.Spec.PoolSize,
			MinIdleConns:         store.Spec.MinIdleConns,
			SentinelMaster:       store.Spec.SentinelMaster,
		})
	}

	return stores
}

func (c *gatewayCompiler) buildPolicyBindings(routes []ir.LogicalRoute, upstreamOrder []string) []ir.LogicalPolicyBinding {
	routeNames := make(map[string]bool, len(routes))
	routeRuleNames := make(map[string]map[string]bool, len(routes))
	for _, route := range routes {
		routeNames[route.Name] = true
		routeRuleNames[route.Name] = make(map[string]bool, len(route.Rules))
		for _, rule := range route.Rules {
			routeRuleNames[route.Name][rule.Name] = true
		}
	}
	upstreamNames := make(map[string]bool, len(upstreamOrder))
	for _, upstreamName := range upstreamOrder {
		upstreamNames[upstreamName] = true
	}

	bindings := make([]ir.LogicalPolicyBinding, 0, len(c.bundle.PolicyBindings))
	for _, binding := range c.bundle.PolicyBindings {
		if !binding.Spec.Enabled {
			continue
		}
		target := binding.Spec.TargetRef
		if target.Kind == resource.KindGateway && target.Name != c.gatewayName {
			continue
		}
		if target.Kind == resource.KindRoute && !routeNames[target.Name] {
			continue
		}
		if target.Kind == resource.KindRoute && target.RuleName != "" && !routeRuleNames[target.Name][target.RuleName] {
			continue
		}
		if target.Kind == resource.KindUpstream && !upstreamNames[target.Name] {
			continue
		}

		logicalBinding := ir.LogicalPolicyBinding{
			Name: binding.Name,
			Target: ir.LogicalPolicyTarget{
				Kind:     target.Kind,
				Name:     target.Name,
				RuleName: target.RuleName,
			},
			Policies: make([]ir.LogicalPolicyRef, 0, len(binding.Spec.Policies)),
		}
		for _, policy := range binding.Spec.Policies {
			if policy.Kind == resource.KindRateLimitPolicy {
				rateLimitPolicy := c.rateLimitPoliciesByName[policy.Name]
				if !rateLimitPolicy.Spec.Enabled {
					continue
				}
			}
			logicalBinding.Policies = append(logicalBinding.Policies, ir.LogicalPolicyRef{
				Kind: policy.Kind,
				Name: policy.Name,
			})
		}
		if len(logicalBinding.Policies) == 0 {
			continue
		}
		bindings = append(bindings, logicalBinding)
	}

	return bindings
}

func (c *gatewayCompiler) buildPluginBindings(
	routes []ir.LogicalRoute,
	upstreamOrder []string,
	aiRoutes attachedAIRoutes,
) []ir.LogicalPluginBinding {
	routeNames := make(map[string]bool, len(routes))
	for _, route := range routes {
		routeNames[route.Name] = true
	}
	upstreamNames := make(map[string]bool, len(upstreamOrder))
	for _, upstreamName := range upstreamOrder {
		upstreamNames[upstreamName] = true
	}
	aiRouteNames := make(map[string]bool, len(aiRoutes.routes))
	for _, route := range aiRoutes.routes {
		aiRouteNames[route.Name] = true
	}
	aiProviderNames := make(map[string]bool, len(aiRoutes.providerOrder))
	for _, providerName := range aiRoutes.providerOrder {
		aiProviderNames[providerName] = true
	}
	aiModelNames := make(map[string]bool, len(aiRoutes.modelOrder))
	for _, modelName := range aiRoutes.modelOrder {
		aiModelNames[modelName] = true
	}

	bindings := make([]ir.LogicalPluginBinding, 0, len(c.bundle.PluginBindings))
	for _, binding := range c.bundle.PluginBindings {
		target := binding.Spec.TargetRef
		if target.Kind == resource.KindGateway && target.Name != c.gatewayName {
			continue
		}
		if target.Kind == resource.KindRoute && !routeNames[target.Name] {
			continue
		}
		if target.Kind == resource.KindUpstream && !upstreamNames[target.Name] {
			continue
		}
		if target.Kind == resource.KindAIRoute && !aiRouteNames[target.Name] {
			continue
		}
		if target.Kind == resource.KindAIProvider && !aiProviderNames[target.Name] {
			continue
		}
		if target.Kind == resource.KindAIModel && !aiModelNames[target.Name] {
			continue
		}

		logicalBinding := ir.LogicalPluginBinding{
			Name: binding.Name,
			Target: ir.LogicalPluginTarget{
				Kind: target.Kind,
				Name: target.Name,
			},
			Phase:         binding.Spec.Phase,
			Priority:      binding.Spec.Priority,
			FailurePolicy: binding.Spec.FailurePolicy,
			Plugins:       make([]ir.LogicalPluginRef, 0, len(binding.Spec.Plugins)),
		}
		for _, plugin := range binding.Spec.Plugins {
			logicalBinding.Plugins = append(logicalBinding.Plugins, ir.LogicalPluginRef{
				Name:   plugin.Name,
				Config: slices.Clone(plugin.Config.Raw),
			})
		}
		bindings = append(bindings, logicalBinding)
	}

	return bindings
}
