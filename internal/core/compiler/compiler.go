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
	upstreamsByName         map[string]resource.Upstream
	authPoliciesByName      map[string]resource.AuthPolicy
	rateLimitPoliciesByName map[string]resource.RateLimitPolicy
	redisStoresByName       map[string]resource.RedisStore
	routeRulesByRoute       map[string]map[string]bool
	policyBindingsByName    map[string]bool
}

// CompileGateway 从内存资源集合中编译指定 Gateway
func (Compiler) CompileGateway(bundle resource.Bundle, gatewayName string) (ir.LogicalGateway, error) {
	c := gatewayCompiler{
		bundle:                  bundle,
		gatewayName:             gatewayName,
		gatewaysByName:          make(map[string]resource.Gateway, len(bundle.Gateways)),
		routesByName:            make(map[string]bool, len(bundle.Routes)),
		upstreamsByName:         make(map[string]resource.Upstream, len(bundle.Upstreams)),
		authPoliciesByName:      make(map[string]resource.AuthPolicy, len(bundle.AuthPolicies)),
		rateLimitPoliciesByName: make(map[string]resource.RateLimitPolicy, len(bundle.RateLimitPolicies)),
		redisStoresByName:       make(map[string]resource.RedisStore, len(bundle.RedisStores)),
		routeRulesByRoute:       make(map[string]map[string]bool, len(bundle.Routes)),
		policyBindingsByName:    make(map[string]bool, len(bundle.PolicyBindings)),
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
	if err := c.indexUpstreams(); err != nil {
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

	routes, upstreamOrder, err := c.buildAttachedRoutes()
	if err != nil {
		return ir.LogicalGateway{}, err
	}
	policyBindings := c.buildPolicyBindings(routes)
	rateLimitPolicies, redisStoreNames := c.buildRateLimitPolicies(policyBindings)

	return ir.LogicalGateway{
		Name:              c.gateway.Name,
		Listeners:         c.buildListeners(),
		Routes:            routes,
		Upstreams:         c.buildUsedUpstreams(upstreamOrder),
		AuthPolicies:      c.buildAuthPolicies(policyBindings),
		RateLimitPolicies: rateLimitPolicies,
		RedisStores:       c.buildRedisStores(redisStoreNames),
		PolicyBindings:    policyBindings,
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

func (c *gatewayCompiler) indexUpstreams() error {
	for _, upstream := range c.bundle.Upstreams {
		if _, ok := c.upstreamsByName[upstream.Name]; ok {
			return fmt.Errorf("duplicate upstream %q", upstream.Name)
		}
		c.upstreamsByName[upstream.Name] = upstream
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

func (c *gatewayCompiler) buildPolicyBindings(routes []ir.LogicalRoute) []ir.LogicalPolicyBinding {
	routeNames := make(map[string]bool, len(routes))
	routeRuleNames := make(map[string]map[string]bool, len(routes))
	for _, route := range routes {
		routeNames[route.Name] = true
		routeRuleNames[route.Name] = make(map[string]bool, len(route.Rules))
		for _, rule := range route.Rules {
			routeRuleNames[route.Name][rule.Name] = true
		}
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
