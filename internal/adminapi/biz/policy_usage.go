package biz

import (
	"context"
	"slices"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// RateLimitPolicyLister 定义策略引用检查需要的限流策略分页能力
type RateLimitPolicyLister interface {
	ListPage(context.Context, PageRequest) (PageResult[resource.RateLimitPolicy], error)
}

// AccessControlPolicyLister 定义策略引用检查需要的访问控制策略分页能力
type AccessControlPolicyLister interface {
	ListPage(context.Context, PageRequest) (PageResult[resource.AccessControlPolicy], error)
}

// TokenQuotaPolicyLister 定义策略引用检查需要的 Token 配额策略分页能力
type TokenQuotaPolicyLister interface {
	ListPage(context.Context, PageRequest) (PageResult[resource.TokenQuotaPolicy], error)
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
	var usage *PolicyUsage
	err := VisitPages(ctx, f.rateLimitPolicies.ListPage, func(policy resource.RateLimitPolicy) (bool, error) {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			usage = &PolicyUsage{DisplayName: policy.Spec.DisplayName}
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	if usage != nil {
		return usage, nil
	}

	err = VisitPages(ctx, f.accessControlPolicies.ListPage, func(policy resource.AccessControlPolicy) (bool, error) {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			usage = &PolicyUsage{DisplayName: policy.Spec.DisplayName}
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	if usage != nil {
		return usage, nil
	}

	err = VisitPages(ctx, f.tokenQuotaPolicies.ListPage, func(policy resource.TokenQuotaPolicy) (bool, error) {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			usage = &PolicyUsage{DisplayName: policy.Spec.DisplayName}
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return usage, nil
}
