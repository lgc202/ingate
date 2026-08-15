package compiler

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	rateLimitHTTPFilterName = "ingate.filters.http.ratelimit"
	rateLimitPluginName     = "ingate.ratelimit"
	rateLimitPluginPath     = "/opt/ingate/plugins/ratelimit.wasm"
	rateLimitRuleName       = "limit"
)

type compiledRateLimitPolicy struct {
	source  ResourceGeneration
	policy  pluginratelimit.Policy
	targets []gatewayv1.PolicyTargetRef
}

func (c *compilation) compileRateLimitPolicies() map[string]compiledRateLimitPolicy {
	result := make(map[string]compiledRateLimitPolicy)
	for _, policyID := range slices.Sorted(maps.Keys(c.rateLimitPolicies)) {
		policy := c.rateLimitPolicies[policyID]
		targets := c.validPolicyTargets(gatewayv1.KindRateLimitPolicy, policyID, policy.Spec.TargetRefs)
		if !policy.Spec.Enabled {
			continue
		}
		pluginPolicy, valid := c.rateLimitPolicy(policy)
		if !valid {
			continue
		}
		result[policyID] = compiledRateLimitPolicy{
			source:  newResourceGeneration(gatewayv1.KindRateLimitPolicy, policy.Name, policy.UID, policy.Generation),
			policy:  pluginPolicy,
			targets: targets,
		}
	}
	return result
}

func (c *compilation) rateLimitPolicy(policy *gatewayv1.RateLimitPolicy) (pluginratelimit.Policy, bool) {
	valid := true
	var key pluginratelimit.KeyPart
	switch policy.Spec.Subject.Type {
	case gatewayv1.RateLimitSubjectShared:
		// Scope 已区分 Gateway 或 Route，Gateway 维度只提供一个稳定的共享 key 值
		key.Type = pluginratelimit.KeyTypeGateway
	case gatewayv1.RateLimitSubjectIP:
		key.Type = pluginratelimit.KeyTypeIP
	case gatewayv1.RateLimitSubjectHeader:
		key.Type = pluginratelimit.KeyTypeHeader
		key.Name = policy.Spec.Subject.HeaderName
		if key.Name == "" || len(k8svalidation.IsHTTPHeaderName(key.Name)) > 0 {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q has an invalid subject header name", policy.Name))
			valid = false
		}
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonUnsupported, fmt.Sprintf("rate limit policy %q uses unsupported subject type %q", policy.Name, policy.Spec.Subject.Type))
		valid = false
	}
	if policy.Spec.Subject.Type != gatewayv1.RateLimitSubjectHeader && policy.Spec.Subject.HeaderName != "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q must not declare a subject header name", policy.Name))
		valid = false
	}
	if policy.Spec.Limit.Requests <= 0 || policy.Spec.Limit.Requests > gatewayv1.RateLimitMaxRequests ||
		policy.Spec.Limit.WindowSeconds <= 0 || policy.Spec.Limit.WindowSeconds > gatewayv1.RateLimitMaxWindowSeconds {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q has an invalid limit", policy.Name))
		valid = false
	}

	return pluginratelimit.Policy{
		Name: policy.Name,
		Rules: []pluginratelimit.Rule{{
			Name: rateLimitRuleName,
			Key:  []pluginratelimit.KeyPart{key},
			Limit: pluginratelimit.Quota{
				Requests:      int(policy.Spec.Limit.Requests),
				WindowSeconds: int(policy.Spec.Limit.WindowSeconds),
			},
		}},
		Response: pluginratelimit.Response{
			QuotaHeaderEnabled: true,
		},
		FailurePolicy: pluginratelimit.FailurePolicyFailOpen,
	}, valid
}

func buildRateLimitHTTPFilter(config *pluginratelimit.PluginConfig) (*hcmv3.HttpFilter, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode rate limit plugin config: %w", err)
	}
	return buildWasmHTTPFilter(
		rateLimitHTTPFilterName,
		rateLimitPluginName,
		rateLimitPluginPath,
		raw,
	)
}
