package compiler

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

type compiledHeaderTransformationPolicy struct {
	source  ResourceGeneration
	filter  wasmFilter
	targets []gatewayv1.PolicyTargetRef
}

type transformerConfig struct {
	RequestRules  []transformerRule `json:"reqRules"`
	ResponseRules []transformerRule `json:"respRules"`
}

type transformerRule struct {
	Operation string              `json:"operate"`
	Headers   []transformerHeader `json:"headers"`
}

type transformerHeader struct {
	Key         string `json:"key,omitempty"`
	OldKey      string `json:"oldKey,omitempty"`
	NewKey      string `json:"newKey,omitempty"`
	Value       string `json:"value,omitempty"`
	NewValue    string `json:"newValue,omitempty"`
	AppendValue string `json:"appendValue,omitempty"`
}

func (c *compilation) compileHeaderTransformationPolicies() map[string]compiledHeaderTransformationPolicy {
	result := make(map[string]compiledHeaderTransformationPolicy)
	for _, policyID := range slices.Sorted(maps.Keys(c.headerTransformationPolicies)) {
		policy := c.headerTransformationPolicies[policyID]
		targets := c.validPolicyTargets(gatewayv1.KindHeaderTransformationPolicy, policyID, policy.Spec.TargetRefs)
		if !policy.Spec.Enabled {
			continue
		}
		plugin, exists := c.wasmPluginsByPackage[gatewayv1.WasmPluginPackageTransformer]
		if !exists {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindHeaderTransformationPolicy,
				policyID,
				ReasonReferenceNotFound,
				fmt.Sprintf("header transformation policy %q requires installed plugin package %q", policyID, gatewayv1.WasmPluginPackageTransformer),
			)
			continue
		}
		module, exists := c.wasmModules[plugin.Name]
		if !exists {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindHeaderTransformationPolicy,
				policyID,
				ReasonInvalidReference,
				fmt.Sprintf("header transformation policy %q cannot use plugin package %q because its module is unavailable", policyID, plugin.Spec.Package),
			)
			continue
		}
		configuration, valid := c.headerTransformationConfiguration(policy)
		if !valid {
			continue
		}
		result[policyID] = compiledHeaderTransformationPolicy{
			source: newResourceGeneration(
				gatewayv1.KindHeaderTransformationPolicy,
				policy.Name,
				policy.UID,
				policy.Generation,
			),
			filter: wasmFilter{
				name:          policy.Name,
				vmID:          plugin.Name,
				rootID:        plugin.Spec.RootID,
				configuration: configuration,
				module:        module,
			},
			targets: targets,
		}
	}
	return result
}

func (c *compilation) headerTransformationConfiguration(policy *gatewayv1.HeaderTransformationPolicy) ([]byte, bool) {
	if len(policy.Spec.RequestRules)+len(policy.Spec.ResponseRules) == 0 {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindHeaderTransformationPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("header transformation policy %q must contain at least one request or response rule", policy.Name),
		)
		return nil, false
	}
	config := transformerConfig{
		RequestRules:  transformerRules(policy.Spec.RequestRules),
		ResponseRules: transformerRules(policy.Spec.ResponseRules),
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindHeaderTransformationPolicy,
			policy.Name,
			ReasonCompileFailed,
			fmt.Sprintf("encode header transformation policy %q: %v", policy.Name, err),
		)
		return nil, false
	}
	return encoded, true
}

func transformerRules(rules []gatewayv1.HeaderTransformationRule) []transformerRule {
	result := make([]transformerRule, 0, len(rules))
	for _, rule := range rules {
		result = append(result, transformerRule{
			Operation: strings.ToLower(string(rule.Operation)),
			Headers:   []transformerHeader{transformerHeaderRule(rule)},
		})
	}
	return result
}

func transformerHeaderRule(rule gatewayv1.HeaderTransformationRule) transformerHeader {
	switch rule.Operation {
	case gatewayv1.HeaderTransformationRemove:
		return transformerHeader{Key: rule.Name}
	case gatewayv1.HeaderTransformationRename:
		return transformerHeader{OldKey: rule.Name, NewKey: rule.Value}
	case gatewayv1.HeaderTransformationReplace:
		return transformerHeader{Key: rule.Name, NewValue: rule.Value}
	case gatewayv1.HeaderTransformationAdd:
		return transformerHeader{Key: rule.Name, Value: rule.Value}
	case gatewayv1.HeaderTransformationAppend:
		return transformerHeader{Key: rule.Name, AppendValue: rule.Value}
	default:
		return transformerHeader{Key: rule.Name}
	}
}

func matchingHeaderTransformationPolicies(
	policies map[string]compiledHeaderTransformationPolicy,
	key policyRouteKey,
) ([]compiledHeaderTransformationPolicy, []matchedPolicyTarget) {
	matched := make([]compiledHeaderTransformationPolicy, 0)
	targets := make([]matchedPolicyTarget, 0)
	for _, policyID := range slices.Sorted(maps.Keys(policies)) {
		compiled := policies[policyID]
		_, matchedTargets := matchingPolicyTargets(compiled.targets, key)
		if len(matchedTargets) == 0 {
			continue
		}
		matched = append(matched, compiled)
		targets = append(targets, matchedPolicyTarget{source: compiled.source, targets: matchedTargets})
	}
	return matched, targets
}

func applyHeaderTransformationPolicies(
	routes []*routev3.Route,
	policies []compiledHeaderTransformationPolicy,
	config *listenerFilterConfig,
) error {
	for _, policy := range policies {
		filterName := wasmFilterName(policy.filter.name)
		for _, route := range routes {
			if err := enableWasmOnRoute(route, filterName); err != nil {
				return err
			}
		}
		if !listenerHasWasmFilter(config.wasm, policy.filter.name) {
			config.wasm = append(config.wasm, policy.filter)
		}
	}
	return nil
}

func listenerHasWasmFilter(filters []wasmFilter, name string) bool {
	for _, filter := range filters {
		if filter.name == name {
			return true
		}
	}
	return false
}
