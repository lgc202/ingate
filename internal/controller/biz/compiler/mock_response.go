package compiler

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

type compiledMockResponsePolicy struct {
	source  ResourceGeneration
	filter  wasmFilter
	targets []gatewayv1.PolicyTargetRef
}

type mockResponseConfig struct {
	StatusCode uint32               `json:"statusCode"`
	Headers    []mockResponseHeader `json:"headers,omitempty"`
	Body       string               `json:"body,omitempty"`
}

type mockResponseHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (c *compilation) compileMockResponsePolicies() map[string]compiledMockResponsePolicy {
	result := make(map[string]compiledMockResponsePolicy)
	for _, policyID := range slices.Sorted(maps.Keys(c.mockResponsePolicies)) {
		policy := c.mockResponsePolicies[policyID]
		targets := c.validPolicyTargets(gatewayv1.KindMockResponsePolicy, policyID, policy.Spec.TargetRefs)
		if !policy.Spec.Enabled {
			continue
		}
		plugin, exists := c.wasmPluginsByPackage[gatewayv1.WasmPluginPackageMockResponse]
		if !exists {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindMockResponsePolicy,
				policyID,
				ReasonPluginNotInstalled,
				fmt.Sprintf("mock response policy %q requires installed plugin package %q", policyID, gatewayv1.WasmPluginPackageMockResponse),
			)
			continue
		}
		module, exists := c.wasmModules[plugin.Name]
		if !exists {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindMockResponsePolicy,
				policyID,
				ReasonArtifactUnavailable,
				fmt.Sprintf("mock response policy %q cannot use plugin package %q because its module is unavailable", policyID, plugin.Spec.Package),
			)
			continue
		}
		configuration, valid := c.mockResponseConfiguration(policy)
		if !valid {
			continue
		}
		result[policyID] = compiledMockResponsePolicy{
			source: newResourceGeneration(
				gatewayv1.KindMockResponsePolicy,
				policy.Name,
				policy.UID,
				policy.Generation,
			),
			filter: wasmFilter{
				name:          policy.Name,
				phase:         wasmFilterPhaseLocalResponse,
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

func (c *compilation) mockResponseConfiguration(policy *gatewayv1.MockResponsePolicy) ([]byte, bool) {
	headers := make([]mockResponseHeader, 0, len(policy.Spec.Headers)+1)
	headers = append(headers, mockResponseHeader{Name: "content-type", Value: policy.Spec.ContentType})
	for _, header := range policy.Spec.Headers {
		headers = append(headers, mockResponseHeader{Name: header.Name, Value: header.Value})
	}
	encoded, err := json.Marshal(mockResponseConfig{
		StatusCode: uint32(policy.Spec.StatusCode),
		Headers:    headers,
		Body:       policy.Spec.Body,
	})
	if err != nil {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindMockResponsePolicy,
			policy.Name,
			ReasonCompileFailed,
			fmt.Sprintf("encode mock response policy %q: %v", policy.Name, err),
		)
		return nil, false
	}
	return encoded, true
}

func matchingMockResponsePolicies(
	policies map[string]compiledMockResponsePolicy,
	key policyRouteKey,
) ([]compiledMockResponsePolicy, []matchedPolicyTarget) {
	matched := make([]compiledMockResponsePolicy, 0)
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

func applyMockResponsePolicy(
	routes []*routev3.Route,
	policy compiledMockResponsePolicy,
	config *listenerFilterConfig,
) error {
	filterName := wasmFilterName(policy.filter.name)
	for _, route := range routes {
		if err := enableWasmOnRoute(route, filterName); err != nil {
			return err
		}
	}
	if !listenerHasWasmFilter(config.wasm, policy.filter.name) {
		// 模拟响应会终止请求，因此固定追加在其他 Wasm 治理之后、AI ext_proc 和 Router 之前
		config.wasm = append(config.wasm, policy.filter)
	}
	return nil
}
