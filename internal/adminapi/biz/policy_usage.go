package biz

import (
	"context"
	"slices"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// RateLimitPolicyLister 定义策略引用检查需要的限流策略列表能力
type RateLimitPolicyLister interface {
	List(context.Context) ([]resource.RateLimitPolicy, error)
}

// AccessControlPolicyLister 定义策略引用检查需要的访问控制策略列表能力
type AccessControlPolicyLister interface {
	List(context.Context) ([]resource.AccessControlPolicy, error)
}

// TokenQuotaPolicyLister 定义策略引用检查需要的 Token 配额策略列表能力
type TokenQuotaPolicyLister interface {
	List(context.Context) ([]resource.TokenQuotaPolicy, error)
}

// PolicyUsage 表示一个目标当前被哪条策略应用
type PolicyUsage struct {
	DisplayName string
}

// PolicyUsageFinder 查询 Gateway 或 Route 是否仍被策略引用
type PolicyUsageFinder struct {
	rateLimitPolicies     RateLimitPolicyLister
	accessControlPolicies AccessControlPolicyLister
	tokenQuotaPolicies    TokenQuotaPolicyLister
}

// NewPolicyUsageFinder 创建策略目标引用查询器
func NewPolicyUsageFinder(
	rateLimitPolicies RateLimitPolicyLister,
	accessControlPolicies AccessControlPolicyLister,
	tokenQuotaPolicies TokenQuotaPolicyLister,
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
	for _, policy := range rateLimitPolicies {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			return &PolicyUsage{DisplayName: policy.Spec.DisplayName}, nil
		}
	}

	accessControlPolicies, err := f.accessControlPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, policy := range accessControlPolicies {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			return &PolicyUsage{DisplayName: policy.Spec.DisplayName}, nil
		}
	}

	tokenQuotaPolicies, err := f.tokenQuotaPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, policy := range tokenQuotaPolicies {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			return &PolicyUsage{DisplayName: policy.Spec.DisplayName}, nil
		}
	}
	return nil, nil
}
