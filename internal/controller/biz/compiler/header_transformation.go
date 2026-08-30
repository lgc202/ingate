package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/headertransformationconfig"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
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

func (c *compilation) compileHeaderTransformationPolicies() []compiledHeaderTransformationPolicy {
	result := make([]compiledHeaderTransformationPolicy, 0, len(c.headerTransformationPolicies))
	for _, policyID := range slices.Sorted(maps.Keys(c.headerTransformationPolicies)) {
		policy := c.headerTransformationPolicies[policyID]
		targets := c.validPolicyTargets(gatewayv1.KindHeaderTransformationPolicy, policyID, policy.Spec.TargetRefs)
		if !policy.Spec.Enabled {
			continue
		}
		configuration, valid := c.headerTransformationConfiguration(policy)
		if !valid {
			continue
		}
		plugin, exists := c.wasmPluginsByPackage[gatewayv1.WasmPluginPackageTransformer]
		if !exists {
			c.addResourceError(
				gatewayv1.KindHeaderTransformationPolicy,
				policyID,
				ReasonPluginNotInstalled,
				fmt.Sprintf(
					"header transformation policy %q requires installed plugin package %q",
					policyID,
					gatewayv1.WasmPluginPackageTransformer,
				),
			)
			continue
		}
		module, exists := c.wasmModules[plugin.Name]
		if !exists {
			c.addResourceError(
				gatewayv1.KindHeaderTransformationPolicy,
				policyID,
				ReasonArtifactUnavailable,
				fmt.Sprintf(
					"header transformation policy %q cannot use plugin package %q because its module is unavailable",
					policyID,
					plugin.Spec.Package,
				),
			)
			continue
		}
		result = append(result, compiledHeaderTransformationPolicy{
			source: newResourceGeneration(gatewayv1.KindHeaderTransformationPolicy, policy),
			filter: wasmFilter{
				name:          policy.Name,
				phase:         wasmFilterPhaseTrafficMutation,
				vmID:          plugin.Name,
				rootID:        plugin.Spec.RootID,
				configuration: configuration,
				module:        module,
			},
			targets: targets,
		})
	}
	return result
}

func (c *compilation) headerTransformationConfiguration(policy *gatewayv1.HeaderTransformationPolicy) ([]byte, bool) {
	ruleCount := len(policy.Spec.RequestRules) + len(policy.Spec.ResponseRules)
	if ruleCount == 0 {
		c.addResourceError(
			gatewayv1.KindHeaderTransformationPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("header transformation policy %q must contain at least one request or response rule", policy.Name),
		)
		return nil, false
	}
	if ruleCount > headertransformationconfig.MaxRules {
		c.addResourceError(
			gatewayv1.KindHeaderTransformationPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("header transformation policy %q contains too many rules", policy.Name),
		)
		return nil, false
	}

	requestRules, err := compileTransformerRules(policy.Spec.RequestRules)
	if err != nil {
		c.addResourceError(
			gatewayv1.KindHeaderTransformationPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("header transformation policy %q has invalid request rules: %v", policy.Name, err),
		)
		return nil, false
	}
	responseRules, err := compileTransformerRules(policy.Spec.ResponseRules)
	if err != nil {
		c.addResourceError(
			gatewayv1.KindHeaderTransformationPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("header transformation policy %q has invalid response rules: %v", policy.Name, err),
		)
		return nil, false
	}
	config := transformerConfig{
		RequestRules:  requestRules,
		ResponseRules: responseRules,
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		c.addResourceError(
			gatewayv1.KindHeaderTransformationPolicy,
			policy.Name,
			ReasonCompileFailed,
			fmt.Sprintf("encode header transformation policy %q: %v", policy.Name, err),
		)
		return nil, false
	}
	return encoded, true
}

func compileTransformerRules(rules []gatewayv1.HeaderTransformationRule) ([]transformerRule, error) {
	compiled := make([]transformerRule, len(rules))
	for i, rule := range rules {
		header, err := compileTransformerHeader(rule)
		if err != nil {
			return nil, fmt.Errorf("rule %d: %w", i+1, err)
		}
		compiled[i] = transformerRule{
			Operation: strings.ToLower(string(rule.Operation)),
			Headers:   []transformerHeader{header},
		}
	}
	return compiled, nil
}

func compileTransformerHeader(
	rule gatewayv1.HeaderTransformationRule,
) (transformerHeader, error) {
	rule.Name = httpheader.NormalizeName(rule.Name)
	rule.Value = httpheader.NormalizeValue(rule.Value)
	if !httpheader.IsValidName(rule.Name) {
		return transformerHeader{}, fmt.Errorf("invalid header name %q", rule.Name)
	}

	switch rule.Operation {
	case gatewayv1.HeaderTransformationRemove:
		if rule.Value != "" {
			return transformerHeader{}, errors.New("remove operation does not accept a value")
		}
		return transformerHeader{Key: rule.Name}, nil
	case gatewayv1.HeaderTransformationRename:
		rule.Value = httpheader.NormalizeName(rule.Value)
		if !httpheader.IsValidName(rule.Value) {
			return transformerHeader{}, fmt.Errorf("invalid destination header name %q", rule.Value)
		}
		return transformerHeader{OldKey: rule.Name, NewKey: rule.Value}, nil
	case gatewayv1.HeaderTransformationReplace:
		if !httpheader.IsValidValue(rule.Value) {
			return transformerHeader{}, errors.New("invalid replacement header value")
		}
		return transformerHeader{Key: rule.Name, NewValue: rule.Value}, nil
	case gatewayv1.HeaderTransformationAdd:
		if !httpheader.IsValidValue(rule.Value) {
			return transformerHeader{}, errors.New("invalid added header value")
		}
		return transformerHeader{Key: rule.Name, Value: rule.Value}, nil
	case gatewayv1.HeaderTransformationAppend:
		if !httpheader.IsValidValue(rule.Value) {
			return transformerHeader{}, errors.New("invalid appended header value")
		}
		return transformerHeader{Key: rule.Name, AppendValue: rule.Value}, nil
	default:
		return transformerHeader{}, fmt.Errorf("unsupported operation %q", rule.Operation)
	}
}

func matchingHeaderTransformationPolicies(
	policies []compiledHeaderTransformationPolicy,
	key policyRouteKey,
) ([]compiledHeaderTransformationPolicy, []matchedPolicyTarget) {
	matched := make([]compiledHeaderTransformationPolicy, 0)
	targets := make([]matchedPolicyTarget, 0)
	for _, compiled := range policies {
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
