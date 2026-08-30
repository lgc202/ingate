package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/internal/pkg/mockresponseconfig"
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

func (c *compilation) compileMockResponsePolicies() []compiledMockResponsePolicy {
	result := make([]compiledMockResponsePolicy, 0, len(c.mockResponsePolicies))
	for _, policyID := range slices.Sorted(maps.Keys(c.mockResponsePolicies)) {
		policy := c.mockResponsePolicies[policyID]
		targets := c.validPolicyTargets(gatewayv1.KindMockResponsePolicy, policyID, policy.Spec.TargetRefs)
		if !policy.Spec.Enabled {
			continue
		}
		configuration, valid := c.mockResponseConfiguration(policy)
		if !valid {
			continue
		}
		plugin, exists := c.wasmPluginsByPackage[gatewayv1.WasmPluginPackageMockResponse]
		if !exists {
			c.addResourceError(
				gatewayv1.KindMockResponsePolicy,
				policyID,
				ReasonPluginNotInstalled,
				fmt.Sprintf(
					"mock response policy %q requires installed plugin package %q",
					policyID,
					gatewayv1.WasmPluginPackageMockResponse,
				),
			)
			continue
		}
		module, exists := c.wasmModules[plugin.Name]
		if !exists {
			c.addResourceError(
				gatewayv1.KindMockResponsePolicy,
				policyID,
				ReasonArtifactUnavailable,
				fmt.Sprintf(
					"mock response policy %q cannot use plugin package %q because its module is unavailable",
					policyID,
					plugin.Spec.Package,
				),
			)
			continue
		}
		result = append(result, compiledMockResponsePolicy{
			source: newResourceGeneration(gatewayv1.KindMockResponsePolicy, policy),
			filter: wasmFilter{
				name:          policy.Name,
				phase:         wasmFilterPhaseLocalResponse,
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

func (c *compilation) mockResponseConfiguration(policy *gatewayv1.MockResponsePolicy) ([]byte, bool) {
	config, err := buildMockResponseConfig(policy.Spec)
	if err != nil {
		c.addResourceError(
			gatewayv1.KindMockResponsePolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("mock response policy %q is invalid: %v", policy.Name, err),
		)
		return nil, false
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		c.addResourceError(
			gatewayv1.KindMockResponsePolicy,
			policy.Name,
			ReasonCompileFailed,
			fmt.Sprintf("encode mock response policy %q: %v", policy.Name, err),
		)
		return nil, false
	}
	return encoded, true
}

func buildMockResponseConfig(spec gatewayv1.MockResponsePolicySpec) (mockResponseConfig, error) {
	if !mockresponseconfig.IsValidStatusCode(spec.StatusCode) {
		return mockResponseConfig{}, fmt.Errorf("unsupported status code %d", spec.StatusCode)
	}
	contentType, valid := mockresponseconfig.NormalizeContentType(spec.ContentType)
	if !valid {
		return mockResponseConfig{}, errors.New("invalid content type")
	}
	if len(spec.Body) > mockresponseconfig.MaxBodyBytes {
		return mockResponseConfig{}, fmt.Errorf("body exceeds %d bytes", mockresponseconfig.MaxBodyBytes)
	}
	if len(spec.Headers) > mockresponseconfig.MaxHeaders {
		return mockResponseConfig{}, fmt.Errorf("header count exceeds %d", mockresponseconfig.MaxHeaders)
	}

	headers := make([]mockResponseHeader, len(spec.Headers)+1)
	headers[0] = mockResponseHeader{Name: "content-type", Value: contentType}
	seen := make(map[string]bool, len(spec.Headers))
	for i, header := range spec.Headers {
		name := httpheader.NormalizeName(header.Name)
		if !httpheader.IsValidName(name) {
			return mockResponseConfig{}, fmt.Errorf("header %d has invalid name %q", i+1, name)
		}
		if mockresponseconfig.IsReservedHeaderName(name) {
			return mockResponseConfig{}, fmt.Errorf("header %q is reserved", name)
		}
		if seen[name] {
			return mockResponseConfig{}, fmt.Errorf("header %q is duplicated", name)
		}
		seen[name] = true

		value := httpheader.NormalizeValue(header.Value)
		if !httpheader.IsValidValue(value) {
			return mockResponseConfig{}, fmt.Errorf("header %q has invalid value", name)
		}
		headers[i+1] = mockResponseHeader{Name: name, Value: value}
	}
	return mockResponseConfig{
		StatusCode: uint32(spec.StatusCode),
		Headers:    headers,
		Body:       spec.Body,
	}, nil
}

func matchingMockResponsePolicies(
	policies []compiledMockResponsePolicy,
	key policyRouteKey,
) ([]compiledMockResponsePolicy, []matchedPolicyTarget) {
	matched := make([]compiledMockResponsePolicy, 0)
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
