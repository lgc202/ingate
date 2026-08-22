package compiler

import (
	"fmt"
	"maps"
	"slices"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

type policyRouteKey struct {
	listenerKey listenerKey
	gatewayID   string
	routeID     string
}

type matchedPolicyTarget struct {
	source  ResourceGeneration
	targets []gatewayv1.PolicyTargetRef
}

func (c *compilation) buildPolicyConfigs(
	attachments []routeAttachment,
) (map[listenerKey]listenerFilterConfig, []CompiledPolicyTarget) {
	// 编译阶段直接把强类型 Policy 展开到 Envoy Route，执行组件不再理解用户资源挂载关系
	ipRestrictionPolicies := c.compileIPRestrictionPolicies()
	headerTransformationPolicies := c.compileHeaderTransformationPolicies()
	filters := make(map[listenerKey]listenerFilterConfig)
	policyTargetSet := make(map[CompiledPolicyTarget]bool)

	for _, attachment := range attachments {
		key := policyRouteKey{
			listenerKey: attachment.listenerKey,
			gatewayID:   attachment.gatewayID,
			routeID:     attachment.routeID,
		}
		config := filters[key.listenerKey]

		restrictions, restrictionTargets := matchingIPRestrictionPolicies(ipRestrictionPolicies, key)
		if len(restrictions) > 0 {
			if err := applyIPRestrictionPolicies(attachment.routes, restrictions); err != nil {
				c.addDiagnostic(SeverityError, gatewayv1.KindRoute, key.routeID, ReasonCompileFailed, fmt.Sprintf("compile IP restriction policies for route %q: %v", key.routeID, err))
			} else {
				config.ipRestriction = true
				for _, target := range restrictionTargets {
					c.recordPolicyTargets(target.source, target.targets, policyTargetSet)
				}
			}
		}

		transformations, transformationTargets := matchingHeaderTransformationPolicies(headerTransformationPolicies, key)
		if len(transformations) > 0 {
			if err := applyHeaderTransformationPolicies(attachment.routes, transformations, &config); err != nil {
				c.addDiagnostic(SeverityError, gatewayv1.KindRoute, key.routeID, ReasonCompileFailed, fmt.Sprintf("compile header transformation policies for route %q: %v", key.routeID, err))
			} else {
				for _, target := range transformationTargets {
					c.recordPolicyTargets(target.source, target.targets, policyTargetSet)
				}
			}
		}

		if config.ipRestriction || len(config.wasm) > 0 {
			filters[key.listenerKey] = config
		}
	}
	return filters, compiledPolicyTargets(policyTargetSet)
}

func matchingIPRestrictionPolicies(
	policies map[string]compiledIPRestrictionPolicy,
	key policyRouteKey,
) ([]ipRestrictionPolicy, []matchedPolicyTarget) {
	matched := make([]ipRestrictionPolicy, 0)
	targets := make([]matchedPolicyTarget, 0)
	for _, policyID := range slices.Sorted(maps.Keys(policies)) {
		compiled := policies[policyID]
		_, matchedTargets := matchingPolicyTargets(compiled.targets, key)
		if len(matchedTargets) == 0 {
			continue
		}
		matched = append(matched, compiled.policy)
		targets = append(targets, matchedPolicyTarget{source: compiled.source, targets: matchedTargets})
	}
	return matched, targets
}
