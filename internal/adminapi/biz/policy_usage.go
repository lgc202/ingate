package biz

import (
	"context"
	"slices"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// PolicyUsage 表示一个目标当前被哪条策略应用
type PolicyUsage struct {
	DisplayName string
}

// PolicyUsageFinder 查询 Gateway 或 Route 是否仍被策略引用
type PolicyUsageFinder struct {
	rateLimitPolicies     RateLimitPolicyRepository
	accessControlPolicies AccessControlPolicyRepository
	tokenQuotaPolicies    TokenQuotaPolicyRepository
}

// NewPolicyUsageFinder 创建策略目标引用查询器
func NewPolicyUsageFinder(
	rateLimitPolicies RateLimitPolicyRepository,
	accessControlPolicies AccessControlPolicyRepository,
	tokenQuotaPolicies TokenQuotaPolicyRepository,
) *PolicyUsageFinder {
	return &PolicyUsageFinder{
		rateLimitPolicies:     rateLimitPolicies,
		accessControlPolicies: accessControlPolicies,
		tokenQuotaPolicies:    tokenQuotaPolicies,
	}
}

// Find 返回第一条仍引用目标的策略，没有引用时返回 nil
func (f *PolicyUsageFinder) Find(ctx context.Context, target resource.PolicyTargetRef) (*PolicyUsage, error) {
	rateLimitPolicies, err := f.rateLimitPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, policy := range rateLimitPolicies.Items {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			return &PolicyUsage{DisplayName: policy.Spec.DisplayName}, nil
		}
	}

	accessControlPolicies, err := f.accessControlPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, policy := range accessControlPolicies.Items {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			return &PolicyUsage{DisplayName: policy.Spec.DisplayName}, nil
		}
	}

	tokenQuotaPolicies, err := f.tokenQuotaPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, policy := range tokenQuotaPolicies.Items {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			return &PolicyUsage{DisplayName: policy.Spec.DisplayName}, nil
		}
	}
	return nil, nil
}
