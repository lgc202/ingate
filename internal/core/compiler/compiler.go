// Package compiler 将声明式资源编译成运行时无关的 IR
package compiler

import (
	"fmt"
	"slices"

	"github.com/lgc202/ingate-next/internal/core/ir"
	resource "github.com/lgc202/ingate-next/pkg/apis/gateway/v1"
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
	pluginsByName           map[string]resource.Plugin
	authPoliciesByName      map[string]resource.AuthPolicy
	rateLimitPoliciesByName map[string]resource.RateLimitPolicy
	policyBindingsByName    map[string]bool
	pluginBindingsByName    map[string]bool
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
		pluginsByName:           make(map[string]resource.Plugin, len(bundle.Plugins)),
		authPoliciesByName:      make(map[string]resource.AuthPolicy, len(bundle.AuthPolicies)),
		rateLimitPoliciesByName: make(map[string]resource.RateLimitPolicy, len(bundle.RateLimitPolicies)),
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
	if err := c.indexPlugins(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexAuthPolicies(); err != nil {
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
	aiRoutes, aiProviderOrder, err := c.buildAttachedAIRoutes()
	if err != nil {
		return ir.LogicalGateway{}, err
	}
	policyBindings := c.buildPolicyBindings(routes, upstreamOrder)
	pluginBindings := c.buildPluginBindings(routes, upstreamOrder)

	return ir.LogicalGateway{
		Name:              c.gateway.Name,
		Listeners:         c.buildListeners(),
		Routes:            routes,
		AIRoutes:          aiRoutes,
		Upstreams:         c.buildUsedUpstreams(upstreamOrder),
		AIProviders:       c.buildUsedAIProviders(aiProviderOrder),
		Plugins:           c.buildPlugins(pluginBindings),
		AuthPolicies:      c.buildAuthPolicies(policyBindings),
		RateLimitPolicies: c.buildRateLimitPolicies(policyBindings),
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
	c.gateway = gateway

	return nil
}

func (c *gatewayCompiler) indexRoutes() error {
	for _, route := range c.bundle.Routes {
		if c.routesByName[route.Name] {
			return fmt.Errorf("duplicate route %q", route.Name)
		}
		c.routesByName[route.Name] = true
		for _, parentRef := range route.Spec.ParentRefs {
			if _, ok := c.gatewaysByName[parentRef]; !ok {
				return fmt.Errorf("route %q references gateway %q", route.Name, parentRef)
			}
		}
	}

	return nil
}

func (c *gatewayCompiler) indexAIRoutes() error {
	for _, route := range c.bundle.AIRoutes {
		if c.aiRoutesByName[route.Name] {
			return fmt.Errorf("duplicate ai route %q", route.Name)
		}
		c.aiRoutesByName[route.Name] = true
		for _, parentRef := range route.Spec.ParentRefs {
			if _, ok := c.gatewaysByName[parentRef]; !ok {
				return fmt.Errorf("ai route %q references gateway %q", route.Name, parentRef)
			}
		}
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
		c.rateLimitPoliciesByName[policy.Name] = policy
	}

	return nil
}

func (c *gatewayCompiler) indexPolicyBindings() error {
	for _, binding := range c.bundle.PolicyBindings {
		if c.policyBindingsByName[binding.Name] {
			return fmt.Errorf("duplicate policy binding %q", binding.Name)
		}
		c.policyBindingsByName[binding.Name] = true

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
			Protocol: listener.Protocol,
			Port:     listener.Port,
			Hostname: listener.Hostname,
		})
	}
	return listeners
}

func (c *gatewayCompiler) buildAttachedRoutes() ([]ir.LogicalRoute, []string, error) {
	routes := make([]ir.LogicalRoute, 0, len(c.bundle.Routes))
	usedUpstreams := make(map[string]bool)
	var upstreamOrder []string

	for _, route := range c.bundle.Routes {
		if !slices.Contains(route.Spec.ParentRefs, c.gatewayName) {
			continue
		}

		logicalRoute := ir.LogicalRoute{
			Name:      route.Name,
			Hostnames: slices.Clone(route.Spec.Hostnames),
			Rules:     make([]ir.LogicalRouteRule, 0, len(route.Spec.Rules)),
		}
		for _, rule := range route.Spec.Rules {
			logicalRule := ir.LogicalRouteRule{
				PathPrefix:    rule.PathPrefix,
				Methods:       slices.Clone(rule.Methods),
				TimeoutMillis: rule.TimeoutMillis,
				Upstreams:     make([]ir.LogicalUpstreamRef, 0, len(rule.UpstreamRefs)),
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
			logicalRoute.Rules = append(logicalRoute.Rules, logicalRule)
		}
		routes = append(routes, logicalRoute)
	}

	return routes, upstreamOrder, nil
}

func (c *gatewayCompiler) buildAttachedAIRoutes() ([]ir.LogicalAIRoute, []string, error) {
	routes := make([]ir.LogicalAIRoute, 0, len(c.bundle.AIRoutes))
	usedProviders := make(map[string]bool)
	var providerOrder []string

	for _, route := range c.bundle.AIRoutes {
		if !slices.Contains(route.Spec.ParentRefs, c.gatewayName) {
			continue
		}
		if len(route.Spec.ProviderRefs) == 0 {
			return nil, nil, fmt.Errorf("ai route %q has no ai providers", route.Name)
		}

		logicalRoute := ir.LogicalAIRoute{
			Name:       route.Name,
			PathPrefix: route.Spec.PathPrefix,
			Model:      route.Spec.Model,
			Providers:  make([]ir.LogicalAIProviderRef, 0, len(route.Spec.ProviderRefs)),
		}
		for _, providerRef := range route.Spec.ProviderRefs {
			if providerRef.Weight <= 0 {
				return nil, nil, fmt.Errorf("ai route %q provider %q has invalid weight %d", route.Name, providerRef.Name, providerRef.Weight)
			}
			if _, ok := c.aiProvidersByName[providerRef.Name]; !ok {
				return nil, nil, fmt.Errorf("ai route %q references ai provider %q", route.Name, providerRef.Name)
			}
			logicalRoute.Providers = append(logicalRoute.Providers, ir.LogicalAIProviderRef{
				Name:   providerRef.Name,
				Weight: providerRef.Weight,
			})
			if !usedProviders[providerRef.Name] {
				usedProviders[providerRef.Name] = true
				providerOrder = append(providerOrder, providerRef.Name)
			}
		}
		routes = append(routes, logicalRoute)
	}

	return routes, providerOrder, nil
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

func (c *gatewayCompiler) buildRateLimitPolicies(bindings []ir.LogicalPolicyBinding) []ir.LogicalRateLimitPolicy {
	usedPolicies := make(map[string]bool)
	var policyOrder []string

	for _, binding := range bindings {
		for _, policy := range binding.Policies {
			if policy.Kind != resource.KindRateLimitPolicy || usedPolicies[policy.Name] {
				continue
			}
			usedPolicies[policy.Name] = true
			policyOrder = append(policyOrder, policy.Name)
		}
	}

	policies := make([]ir.LogicalRateLimitPolicy, 0, len(policyOrder))
	for _, name := range policyOrder {
		policy := c.rateLimitPoliciesByName[name]
		policies = append(policies, ir.LogicalRateLimitPolicy{
			Name:          policy.Name,
			Requests:      policy.Spec.Requests,
			WindowSeconds: policy.Spec.WindowSeconds,
			KeyBy:         policy.Spec.KeyBy,
			Header:        policy.Spec.Header,
		})
	}

	return policies
}

func (c *gatewayCompiler) buildPolicyBindings(routes []ir.LogicalRoute, upstreamOrder []string) []ir.LogicalPolicyBinding {
	routeNames := make(map[string]bool, len(routes))
	for _, route := range routes {
		routeNames[route.Name] = true
	}
	upstreamNames := make(map[string]bool, len(upstreamOrder))
	for _, upstreamName := range upstreamOrder {
		upstreamNames[upstreamName] = true
	}

	bindings := make([]ir.LogicalPolicyBinding, 0, len(c.bundle.PolicyBindings))
	for _, binding := range c.bundle.PolicyBindings {
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

		logicalBinding := ir.LogicalPolicyBinding{
			Name: binding.Name,
			Target: ir.LogicalPolicyTarget{
				Kind: target.Kind,
				Name: target.Name,
			},
			Policies: make([]ir.LogicalPolicyRef, 0, len(binding.Spec.Policies)),
		}
		for _, policy := range binding.Spec.Policies {
			logicalBinding.Policies = append(logicalBinding.Policies, ir.LogicalPolicyRef{
				Kind: policy.Kind,
				Name: policy.Name,
			})
		}
		bindings = append(bindings, logicalBinding)
	}

	return bindings
}

func (c *gatewayCompiler) buildPluginBindings(routes []ir.LogicalRoute, upstreamOrder []string) []ir.LogicalPluginBinding {
	routeNames := make(map[string]bool, len(routes))
	for _, route := range routes {
		routeNames[route.Name] = true
	}
	upstreamNames := make(map[string]bool, len(upstreamOrder))
	for _, upstreamName := range upstreamOrder {
		upstreamNames[upstreamName] = true
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

		logicalBinding := ir.LogicalPluginBinding{
			Name: binding.Name,
			Target: ir.LogicalPluginTarget{
				Kind: target.Kind,
				Name: target.Name,
			},
			Plugins: make([]ir.LogicalPluginRef, 0, len(binding.Spec.Plugins)),
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
