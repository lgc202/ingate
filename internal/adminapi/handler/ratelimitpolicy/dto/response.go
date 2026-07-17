package dto

import (
	"time"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ratelimitpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimitpolicy"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewListRateLimitPoliciesResp 转换限流策略列表用例结果为控制台响应
func NewListRateLimitPoliciesResp(result *ratelimitpolicyservice.ListResult) ListRateLimitPoliciesResp {
	return ListRateLimitPoliciesResp{
		Policies: lo.Map(result.Policies, func(policy resource.RateLimitPolicy, _ int) RateLimitPolicy {
			return policyFromResource(&policy)
		}),
	}
}

// NewGetRateLimitPolicyResp 转换单个限流策略用例结果为控制台响应
func NewGetRateLimitPolicyResp(result *ratelimitpolicyservice.PolicyResult) RateLimitPolicy {
	return policyFromResource(result.Policy)
}

func policyFromResource(policy *resource.RateLimitPolicy) RateLimitPolicy {
	return RateLimitPolicy{
		ID:      policy.Name,
		Version: policy.ResourceVersion,
		RateLimitPolicyConfig: RateLimitPolicyConfig{
			Name:          policy.Spec.DisplayName,
			Description:   policy.Spec.Description,
			Enabled:       policy.Spec.Enabled,
			Mode:          policy.Spec.Mode,
			Rules:         policy.Spec.Rules,
			Response:      policy.Spec.Response,
			FailurePolicy: policy.Spec.FailurePolicy,
		},
		CreatedAt: createdAt(policy.ObjectMeta),
	}
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
