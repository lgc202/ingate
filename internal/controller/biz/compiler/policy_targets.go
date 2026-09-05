package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/policyconfig"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

func (c *compilation) validPolicyTargets(
	policyKind gatewayv1.Kind,
	policyID string,
	targets []gatewayv1.PolicyTargetRef,
	allowedTargetKinds ...gatewayv1.Kind,
) []gatewayv1.PolicyTargetRef {
	if len(targets) > policyconfig.MaxTargets {
		c.addResourceError(
			policyKind,
			policyID,
			ReasonInvalidSpec,
			fmt.Sprintf("policy %q contains too many targets", policyID),
		)
		targets = targets[:policyconfig.MaxTargets]
	}

	validTargets := make([]gatewayv1.PolicyTargetRef, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		if !slices.Contains(allowedTargetKinds, target.Kind) {
			c.addResourceError(
				policyKind,
				policyID,
				ReasonUnsupported,
				fmt.Sprintf("policy %q targets unsupported kind %q", policyID, target.Kind),
			)
			continue
		}
		if !resourceconfig.IsCanonicalID(target.Name) {
			c.addResourceError(
				policyKind,
				policyID,
				ReasonInvalidSpec,
				fmt.Sprintf("policy %q has invalid target %s %q", policyID, target.Kind, target.Name),
			)
			continue
		}

		key := string(target.Kind) + "\x00" + target.Name
		if seen[key] {
			c.addResourceError(
				policyKind,
				policyID,
				ReasonConflict,
				fmt.Sprintf("policy %q repeats target %s %q", policyID, target.Kind, target.Name),
			)
			continue
		}
		seen[key] = true

		switch target.Kind {
		case gatewayv1.KindGateway:
			if _, exists := c.gateways[target.Name]; !exists {
				c.addResourceWarning(
					policyKind,
					policyID,
					ReasonReferenceNotFound,
					fmt.Sprintf("policy %q references missing gateway %q", policyID, target.Name),
				)
				continue
			}
		case gatewayv1.KindRoute:
			if _, exists := c.routes[target.Name]; !exists {
				c.addResourceWarning(
					policyKind,
					policyID,
					ReasonReferenceNotFound,
					fmt.Sprintf("policy %q references missing route %q", policyID, target.Name),
				)
				continue
			}
		}
		validTargets = append(validTargets, target)
	}
	return validTargets
}

func matchingPolicyTargets(
	targets []gatewayv1.PolicyTargetRef,
	key policyRouteKey,
) (string, []gatewayv1.PolicyTargetRef) {
	// 同一策略同时命中 Gateway 和 Route 时只执行一次，
	// 并让更精确的 Route 范围决定计数作用域。
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
			targetResource = newResourceGeneration(target.Kind, resource)
		} else {
			resource := c.routes[target.Name]
			targetResource = newResourceGeneration(target.Kind, resource)
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
		return cmp.Or(
			compareResourceGeneration(a.Policy, b.Policy),
			compareResourceGeneration(a.Target, b.Target),
		)
	})
	return result
}

func compareResourceGeneration(a, b ResourceGeneration) int {
	return cmp.Or(
		cmp.Compare(a.Kind, b.Kind),
		cmp.Compare(a.Name, b.Name),
		cmp.Compare(string(a.UID), string(b.UID)),
		cmp.Compare(a.Generation, b.Generation),
	)
}
