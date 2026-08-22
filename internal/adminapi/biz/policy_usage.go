package biz

import (
	"context"
	"slices"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// RateLimitPolicyLister 定义策略引用检查需要的限流策略分页能力
type RateLimitPolicyLister interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[resource.RateLimitPolicy], error)
}

// IPRestrictionPolicyLister 定义策略引用检查需要的 IP 访问限制策略分页能力
type IPRestrictionPolicyLister interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[resource.IPRestrictionPolicy], error)
}

// HeaderTransformationPolicyLister 定义策略引用检查需要的 Header 转换策略分页能力
type HeaderTransformationPolicyLister interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[resource.HeaderTransformationPolicy], error)
}

// PolicyUsage 表示一个目标当前被哪条策略应用
type PolicyUsage struct {
	DisplayName string
}

// PolicyUsageFinder 查询 Gateway 或 Route 是否仍被策略引用
type PolicyUsageFinder struct {
	rateLimitPolicies     RateLimitPolicyLister
	ipRestrictionPolicies IPRestrictionPolicyLister
	headerTransformations HeaderTransformationPolicyLister
}

// NewPolicyUsageFinder 创建策略目标引用查询器
func NewPolicyUsageFinder(
	rateLimitPolicies RateLimitPolicyLister,
	ipRestrictionPolicies IPRestrictionPolicyLister,
	headerTransformations HeaderTransformationPolicyLister,
) *PolicyUsageFinder {
	return &PolicyUsageFinder{
		rateLimitPolicies:     rateLimitPolicies,
		ipRestrictionPolicies: ipRestrictionPolicies,
		headerTransformations: headerTransformations,
	}
}

// FindTarget 返回第一条仍引用目标的策略，没有引用时返回 nil
func (f *PolicyUsageFinder) FindTarget(ctx context.Context, target resource.PolicyTargetRef) (*PolicyUsage, error) {
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

	err = VisitPages(ctx, f.ipRestrictionPolicies.ListPage, func(policy resource.IPRestrictionPolicy) (bool, error) {
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

	err = VisitPages(ctx, f.headerTransformations.ListPage, func(policy resource.HeaderTransformationPolicy) (bool, error) {
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
