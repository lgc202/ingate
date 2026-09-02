package policy

import (
	"context"
	"slices"

	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// RateLimitLister 定义策略引用检查需要的限流策略分页能力。
type RateLimitLister interface {
	ListPage(ctx context.Context, page pagination.Request) (pagination.Result[resource.RateLimitPolicy], error)
}

// IPRestrictionLister 定义策略引用检查需要的 IP 访问限制策略分页能力。
type IPRestrictionLister interface {
	ListPage(ctx context.Context, page pagination.Request) (pagination.Result[resource.IPRestrictionPolicy], error)
}

// HeaderTransformationLister 定义策略引用检查需要的 Header 转换策略分页能力。
type HeaderTransformationLister interface {
	ListPage(ctx context.Context, page pagination.Request) (pagination.Result[resource.HeaderTransformationPolicy], error)
}

// MockResponseLister 定义策略引用检查需要的模拟响应策略分页能力。
type MockResponseLister interface {
	ListPage(ctx context.Context, page pagination.Request) (pagination.Result[resource.MockResponsePolicy], error)
}

// Usage 表示一个目标当前被哪条策略应用。
type Usage struct {
	DisplayName string
}

// UsageFinder 查询 Gateway 或 Route 是否仍被策略引用。
type UsageFinder struct {
	rateLimitPolicies     RateLimitLister
	ipRestrictionPolicies IPRestrictionLister
	headerTransformations HeaderTransformationLister
	mockResponses         MockResponseLister
}

// NewUsageFinder 创建策略目标引用查询器。
func NewUsageFinder(
	rateLimitPolicies RateLimitLister,
	ipRestrictionPolicies IPRestrictionLister,
	headerTransformations HeaderTransformationLister,
	mockResponses MockResponseLister,
) *UsageFinder {
	return &UsageFinder{
		rateLimitPolicies:     rateLimitPolicies,
		ipRestrictionPolicies: ipRestrictionPolicies,
		headerTransformations: headerTransformations,
		mockResponses:         mockResponses,
	}
}

// FindTarget 返回第一条仍引用目标的策略，没有引用时返回 nil。
func (f *UsageFinder) FindTarget(
	ctx context.Context,
	target resource.PolicyTargetRef,
) (*Usage, error) {
	usage, err := findPolicyUsage(
		ctx,
		target,
		f.rateLimitPolicies.ListPage,
		func(policy resource.RateLimitPolicy) (string, []resource.PolicyTargetRef) {
			return policy.Spec.DisplayName, policy.Spec.TargetRefs
		},
	)
	if err != nil || usage != nil {
		return usage, err
	}

	usage, err = findPolicyUsage(
		ctx,
		target,
		f.ipRestrictionPolicies.ListPage,
		func(policy resource.IPRestrictionPolicy) (string, []resource.PolicyTargetRef) {
			return policy.Spec.DisplayName, policy.Spec.TargetRefs
		},
	)
	if err != nil || usage != nil {
		return usage, err
	}

	usage, err = findPolicyUsage(
		ctx,
		target,
		f.headerTransformations.ListPage,
		func(policy resource.HeaderTransformationPolicy) (string, []resource.PolicyTargetRef) {
			return policy.Spec.DisplayName, policy.Spec.TargetRefs
		},
	)
	if err != nil || usage != nil {
		return usage, err
	}

	return findPolicyUsage(
		ctx,
		target,
		f.mockResponses.ListPage,
		func(policy resource.MockResponsePolicy) (string, []resource.PolicyTargetRef) {
			return policy.Spec.DisplayName, policy.Spec.TargetRefs
		},
	)
}

func findPolicyUsage[P any](
	ctx context.Context,
	target resource.PolicyTargetRef,
	list func(context.Context, pagination.Request) (pagination.Result[P], error),
	attributesOf func(P) (string, []resource.PolicyTargetRef),
) (*Usage, error) {
	var usage *Usage
	err := pagination.VisitPages(ctx, list, func(policy P) (bool, error) {
		displayName, targetRefs := attributesOf(policy)
		if !slices.Contains(targetRefs, target) {
			return false, nil
		}
		usage = &Usage{DisplayName: displayName}
		return true, nil
	})
	return usage, err
}
