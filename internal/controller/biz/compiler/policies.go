package compiler

import (
	"maps"
	"slices"

	pluginiprestriction "github.com/lgc202/ingate/pkg/plugin/iprestriction"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

type policyRouteKey struct {
	listenerKey listenerKey
	gatewayID   string
	routeID     string
}

func (c *compilation) buildPolicyConfigs(
	attachments []routeAttachment,
) (map[listenerKey]listenerFilterConfig, []CompiledPolicyTarget) {
	// 编译阶段把 Gateway/Route 应用范围展开成执行清单，Wasm 不再理解用户层策略挂载模型
	ipRestrictionPolicies := c.compileIPRestrictionPolicies()
	rateLimitPolicies := c.compileRateLimitPolicies()
	result := make(map[listenerKey]listenerFilterConfig)
	policyTargetSet := make(map[CompiledPolicyTarget]bool)

	routeKeySet := make(map[policyRouteKey]bool)
	for _, attachment := range attachments {
		routeKeySet[policyRouteKey(attachment)] = true
	}
	routeKeys := slices.Collect(maps.Keys(routeKeySet))
	slices.SortFunc(routeKeys, comparePolicyRouteKeys)

	for _, key := range routeKeys {
		restrictionPolicies := make([]pluginiprestriction.Policy, 0)
		for _, policyID := range slices.Sorted(maps.Keys(ipRestrictionPolicies)) {
			compiled := ipRestrictionPolicies[policyID]
			_, matchedTargets := matchingPolicyTargets(compiled.targets, key)
			if len(matchedTargets) == 0 {
				continue
			}
			c.recordPolicyTargets(compiled.source, matchedTargets, policyTargetSet)
			restrictionPolicies = append(restrictionPolicies, compiled.policy)
		}

		ratePolicies := make([]pluginratelimit.Policy, 0)
		for _, policyID := range slices.Sorted(maps.Keys(rateLimitPolicies)) {
			compiled := rateLimitPolicies[policyID]
			scope, matchedTargets := matchingPolicyTargets(compiled.targets, key)
			if len(matchedTargets) == 0 {
				continue
			}
			c.recordPolicyTargets(compiled.source, matchedTargets, policyTargetSet)
			policy := compiled.policy
			policy.Scope = scope
			ratePolicies = append(ratePolicies, policy)
		}

		config := result[key.listenerKey]
		if len(restrictionPolicies) > 0 {
			if config.ipRestriction == nil {
				config.ipRestriction = &pluginiprestriction.PluginConfig{}
			}
			config.ipRestriction.Routes = append(config.ipRestriction.Routes, pluginiprestriction.RouteConfig{
				GatewayName: key.gatewayID,
				RouteName:   key.routeID,
				Policies:    restrictionPolicies,
			})
		}
		if len(ratePolicies) > 0 {
			if config.rateLimit == nil {
				config.rateLimit = &pluginratelimit.PluginConfig{}
			}
			config.rateLimit.Routes = append(config.rateLimit.Routes, pluginratelimit.RouteConfig{
				GatewayName: key.gatewayID,
				RouteName:   key.routeID,
				Policies:    ratePolicies,
			})
		}
		if len(restrictionPolicies) > 0 || len(ratePolicies) > 0 {
			result[key.listenerKey] = config
		}
	}
	return result, compiledPolicyTargets(policyTargetSet)
}

func comparePolicyRouteKeys(a, b policyRouteKey) int {
	if result := compareListenerKeys(a.listenerKey, b.listenerKey); result != 0 {
		return result
	}
	if a.gatewayID != b.gatewayID {
		if a.gatewayID < b.gatewayID {
			return -1
		}
		return 1
	}
	if a.routeID < b.routeID {
		return -1
	}
	if a.routeID > b.routeID {
		return 1
	}
	return 0
}
