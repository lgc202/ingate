package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func (c *compilation) validPolicyTargets(
	policyKind gatewayv1.Kind,
	policyID string,
	targets []gatewayv1.PolicyTargetRef,
) []gatewayv1.PolicyTargetRef {
	validTargets := make([]gatewayv1.PolicyTargetRef, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		key := string(target.Kind) + "\x00" + target.Name
		if seen[key] {
			c.addDiagnostic(
				SeverityError,
				policyKind,
				policyID,
				ReasonConflict,
				fmt.Sprintf("policy %q repeats target %s %q", policyID, target.Kind, target.Name),
			)
			continue
		}
		seen[key] = true

		if target.Name == "" {
			c.addDiagnostic(SeverityError, policyKind, policyID, ReasonInvalidSpec, fmt.Sprintf("policy %q has a target without a name", policyID))
			continue
		}
		switch target.Kind {
		case gatewayv1.KindGateway:
			if _, exists := c.gateways[target.Name]; !exists {
				c.addDiagnostic(
					SeverityWarning,
					policyKind,
					policyID,
					ReasonReferenceNotFound,
					fmt.Sprintf("policy %q references missing gateway %q", policyID, target.Name),
				)
				continue
			}
		case gatewayv1.KindRoute:
			if _, exists := c.routes[target.Name]; !exists {
				c.addDiagnostic(
					SeverityWarning,
					policyKind,
					policyID,
					ReasonReferenceNotFound,
					fmt.Sprintf("policy %q references missing route %q", policyID, target.Name),
				)
				continue
			}
		default:
			c.addDiagnostic(
				SeverityError,
				policyKind,
				policyID,
				ReasonUnsupported,
				fmt.Sprintf("policy %q targets unsupported kind %q", policyID, target.Kind),
			)
			continue
		}
		validTargets = append(validTargets, target)
	}
	return validTargets
}

func matchingPolicyTargets(targets []gatewayv1.PolicyTargetRef, key policyRouteKey) (string, []gatewayv1.PolicyTargetRef) {
	// 同一策略同时命中 Gateway 和 Route 时只执行一次，并让更精确的 Route 范围决定计数作用域
	matched := make([]gatewayv1.PolicyTargetRef, 0, 2)
	scope := ""
	for _, target := range targets {
		switch {
		case target.Kind == gatewayv1.KindRoute && target.Name == key.routeID:
			matched = append(matched, target)
			scope = policyScope(target)
		case target.Kind == gatewayv1.KindGateway && target.Name == key.gatewayID:
			matched = append(matched, target)
			if scope == "" {
				scope = policyScope(target)
			}
		}
	}
	if len(matched) > 0 {
		return scope, matched
	}
	return "", nil
}

func policyScope(target gatewayv1.PolicyTargetRef) string {
	return string(target.Kind) + "/" + target.Name
}

func (c *compilation) recordPolicyTargets(
	policy ResourceGeneration,
	targets []gatewayv1.PolicyTargetRef,
	policyTargetSet map[CompiledPolicyTarget]bool,
) {
	for _, target := range targets {
		var targetResource ResourceGeneration
		if target.Kind == gatewayv1.KindGateway {
			resource := c.gateways[target.Name]
			targetResource = newResourceGeneration(target.Kind, resource.Name, resource.UID, resource.Generation)
		} else {
			resource := c.routes[target.Name]
			targetResource = newResourceGeneration(target.Kind, resource.Name, resource.UID, resource.Generation)
		}
		policyTargetSet[CompiledPolicyTarget{
			Policy: policy,
			Target: targetResource,
		}] = true
	}
}

func compiledPolicyTargets(policyTargetSet map[CompiledPolicyTarget]bool) []CompiledPolicyTarget {
	result := slices.Collect(maps.Keys(policyTargetSet))
	slices.SortFunc(result, func(a, b CompiledPolicyTarget) int {
		if result := compareResourceGeneration(a.Policy, b.Policy); result != 0 {
			return result
		}
		return compareResourceGeneration(a.Target, b.Target)
	})
	return result
}

func compareResourceGeneration(a, b ResourceGeneration) int {
	if result := cmp.Compare(a.Kind, b.Kind); result != 0 {
		return result
	}
	if result := cmp.Compare(a.Name, b.Name); result != 0 {
		return result
	}
	if result := cmp.Compare(string(a.UID), string(b.UID)); result != 0 {
		return result
	}
	return cmp.Compare(a.Generation, b.Generation)
}
