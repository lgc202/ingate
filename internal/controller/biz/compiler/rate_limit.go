package compiler

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"golang.org/x/net/http/httpguts"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/extauthz"
)

type compiledRateLimitPolicy struct {
	source  ResourceGeneration
	rule    extauthz.RateLimitRule
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
		rule, valid := c.rateLimitRule(policy)
		if !valid {
			continue
		}
		result[policyID] = compiledRateLimitPolicy{
			source:  newResourceGeneration(gatewayv1.KindRateLimitPolicy, policy.Name, policy.UID, policy.Generation),
			rule:    rule,
			targets: targets,
		}
	}
	return result
}

func (c *compilation) rateLimitRule(policy *gatewayv1.RateLimitPolicy) (extauthz.RateLimitRule, bool) {
	rule := extauthz.RateLimitRule{
		PolicyID:      policy.Name,
		Subject:       extauthz.RateLimitSubject(policy.Spec.Subject.Type),
		HeaderName:    strings.ToLower(strings.TrimSpace(policy.Spec.Subject.HeaderName)),
		Requests:      policy.Spec.Limit.Requests,
		WindowSeconds: policy.Spec.Limit.WindowSeconds,
	}
	valid := true
	if rule.Requests < 1 || rule.Requests > gatewayv1.RateLimitMaxRequests {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q has invalid request limit", policy.Name))
		valid = false
	}
	if rule.WindowSeconds < 1 || rule.WindowSeconds > gatewayv1.RateLimitMaxWindowSeconds {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q has invalid window", policy.Name))
		valid = false
	}
	switch rule.Subject {
	case extauthz.RateLimitSubjectShared, extauthz.RateLimitSubjectIP:
		if rule.HeaderName != "" {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q only accepts headerName for Header subject", policy.Name))
			valid = false
		}
	case extauthz.RateLimitSubjectHeader:
		if !httpguts.ValidHeaderFieldName(rule.HeaderName) {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonInvalidSpec, fmt.Sprintf("rate limit policy %q has invalid subject header", policy.Name))
			valid = false
		}
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, policy.Name, ReasonUnsupported, fmt.Sprintf("rate limit policy %q uses unsupported subject %q", policy.Name, policy.Spec.Subject.Type))
		valid = false
	}
	return rule, valid
}

func matchingRateLimitPolicies(
	policies map[string]compiledRateLimitPolicy,
	key policyRouteKey,
) ([]extauthz.RateLimitRule, []matchedPolicyTarget) {
	rules := make([]extauthz.RateLimitRule, 0)
	targets := make([]matchedPolicyTarget, 0)
	for _, policyID := range slices.Sorted(maps.Keys(policies)) {
		compiled := policies[policyID]
		scope, matchedTargets := matchingPolicyTargets(compiled.targets, key)
		if len(matchedTargets) == 0 {
			continue
		}
		rule := compiled.rule
		rule.Scope = scope
		rules = append(rules, rule)
		targets = append(targets, matchedPolicyTarget{source: compiled.source, targets: matchedTargets})
	}
	return rules, targets
}

func applyRateLimitPolicies(routes []*routev3.Route, rules []extauthz.RateLimitRule) error {
	encoded, err := extauthz.EncodeRateLimitRules(rules)
	if err != nil {
		return err
	}
	return applyAuthzContext(routes, map[string]string{extauthz.RateLimitsContext: encoded})
}
