package ratelimitpolicy

import (
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	ratelimitpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimitpolicy"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultRejectedStatusCode = 429
	defaultRejectedMessage    = "Too many requests"
	defaultFailurePolicy      = FailurePolicyFailOpen
)

// NewListRateLimitPoliciesResp 转换限流策略列表用例结果为控制台响应
func NewListRateLimitPoliciesResp(result *ratelimitpolicyservice.ListResult) ListRateLimitPoliciesResp {
	policies := make([]RateLimitPolicy, 0, len(result.Policies))
	for i := range result.Policies {
		policies = append(policies, policyFromResource(&result.Policies[i], result.TargetNames))
	}
	return ListRateLimitPoliciesResp{Policies: policies}
}

// NewGetRateLimitPolicyResp 转换单个限流策略用例结果为控制台响应
func NewGetRateLimitPolicyResp(result *ratelimitpolicyservice.PolicyResult) RateLimitPolicy {
	return policyFromResource(result.Policy, result.TargetNames)
}

func policyFromResource(policy *resource.RateLimitPolicy, targetNames policytarget.DisplayNames) RateLimitPolicy {
	status := admindto.NewPolicyStatus(policy.Generation, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := !policy.Spec.Enabled && admindto.ConfigurationApplied(policy.Generation, policy.Status.Conditions)
	if disabled {
		status = admindto.NewDisabledPolicyStatus()
	}
	failurePolicy := FailurePolicy(policy.Spec.FailurePolicy)
	if failurePolicy == "" {
		failurePolicy = defaultFailurePolicy
	}
	return RateLimitPolicy{
		ID:            policy.Name,
		Version:       strconv.FormatInt(policy.Generation, 10),
		Status:        status,
		Name:          policy.Spec.DisplayName,
		Description:   policy.Spec.Description,
		Enabled:       policy.Spec.Enabled,
		Targets:       policyTargets(policy, targetNames, disabled),
		Rules:         rulesFromResource(policy.Spec.Rules),
		Response:      responseFromResource(policy.Spec.Response),
		FailurePolicy: failurePolicy,
		CreatedAt:     createdAt(policy.ObjectMeta),
	}
}

func policyTargets(
	policy *resource.RateLimitPolicy,
	targetNames policytarget.DisplayNames,
	disabled bool,
) []admindto.PolicyTarget {
	targets := make([]admindto.PolicyTarget, 0, len(policy.Spec.TargetRefs))
	for _, ref := range policy.Spec.TargetRefs {
		status := admindto.NewPolicyTargetStatus(policy.Generation, targetConditions(policy.Status.Targets, ref))
		if disabled {
			status = admindto.NewDisabledResourceStatus()
		}
		targets = append(targets, admindto.PolicyTarget{
			Kind:        admindto.PolicyTargetKind(ref.Kind),
			ID:          ref.Name,
			DisplayName: targetNames.Name(ref),
			Status:      status,
		})
	}
	return targets
}

func targetConditions(statuses []resource.PolicyTargetStatus, ref resource.PolicyTargetRef) []metav1.Condition {
	for _, status := range statuses {
		if status.TargetRef.Kind == ref.Kind && status.TargetRef.Name == ref.Name {
			return status.Conditions
		}
	}
	return nil
}

func rulesFromResource(items []resource.RateLimitRule) []Rule {
	rules := make([]Rule, 0, len(items))
	for _, item := range items {
		parts := make([]KeyPart, 0, len(item.Key.Parts))
		for _, part := range item.Key.Parts {
			parts = append(parts, KeyPart{Type: KeyType(part.Type), Name: part.Name})
		}
		rules = append(rules, Rule{
			Name: item.Name,
			Key:  Key{Parts: parts},
			Limit: Quota{
				Requests:      item.Limit.Requests,
				WindowSeconds: item.Limit.WindowSeconds,
				Burst:         item.Limit.Burst,
			},
		})
	}
	return rules
}

func responseFromResource(response resource.RateLimitResponse) Response {
	result := Response{
		StatusCode:         response.StatusCode,
		Message:            response.Message,
		QuotaHeaderEnabled: response.QuotaHeaderEnabled,
	}
	if result.StatusCode == 0 {
		result.StatusCode = defaultRejectedStatusCode
	}
	if result.Message == "" {
		result.Message = defaultRejectedMessage
	}
	return result
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
