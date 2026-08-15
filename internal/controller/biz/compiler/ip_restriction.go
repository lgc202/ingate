package compiler

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/netip"
	"slices"

	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	pluginiprestriction "github.com/lgc202/ingate/pkg/plugin/iprestriction"
)

const (
	ipRestrictionHTTPFilterName = "ingate.filters.http.iprestriction"
	ipRestrictionPluginName     = "ingate.iprestriction"
	ipRestrictionPluginPath     = "/opt/ingate/plugins/iprestriction.wasm"
)

type compiledIPRestrictionPolicy struct {
	source  ResourceGeneration
	policy  pluginiprestriction.Policy
	targets []gatewayv1.PolicyTargetRef
}

func (c *compilation) compileIPRestrictionPolicies() map[string]compiledIPRestrictionPolicy {
	result := make(map[string]compiledIPRestrictionPolicy)
	for _, policyID := range slices.Sorted(maps.Keys(c.ipRestrictionPolicies)) {
		policy := c.ipRestrictionPolicies[policyID]
		targets := c.validPolicyTargets(gatewayv1.KindIPRestrictionPolicy, policyID, policy.Spec.TargetRefs)
		if !policy.Spec.Enabled {
			continue
		}
		pluginPolicy, valid := c.ipRestrictionPolicy(policy)
		if !valid {
			continue
		}
		result[policyID] = compiledIPRestrictionPolicy{
			source:  newResourceGeneration(gatewayv1.KindIPRestrictionPolicy, policy.Name, policy.UID, policy.Generation),
			policy:  pluginPolicy,
			targets: targets,
		}
	}
	return result
}

func (c *compilation) ipRestrictionPolicy(policy *gatewayv1.IPRestrictionPolicy) (pluginiprestriction.Policy, bool) {
	valid := true
	if (len(policy.Spec.Allow) > 0) == (len(policy.Spec.Deny) > 0) {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindIPRestrictionPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("IP restriction policy %q must configure exactly one of allow or deny", policy.Name),
		)
		valid = false
	}
	for _, value := range append(slices.Clone(policy.Spec.Allow), policy.Spec.Deny...) {
		if _, err := netip.ParsePrefix(value); err == nil {
			continue
		}
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindIPRestrictionPolicy,
			policy.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("IP restriction policy %q contains invalid IP prefix %q", policy.Name, value),
		)
		valid = false
	}
	return pluginiprestriction.Policy{
		Name:  policy.Name,
		Allow: slices.Clone(policy.Spec.Allow),
		Deny:  slices.Clone(policy.Spec.Deny),
	}, valid
}

func buildIPRestrictionHTTPFilter(config *pluginiprestriction.PluginConfig) (*hcmv3.HttpFilter, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode IP restriction plugin config: %w", err)
	}
	return buildWasmHTTPFilter(
		ipRestrictionHTTPFilterName,
		ipRestrictionPluginName,
		ipRestrictionPluginPath,
		raw,
	)
}
