package policytarget

import (
	"context"
	"slices"

	accesscontrolpolicystore "github.com/lgc202/ingate/internal/admin/store/accesscontrolpolicy"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/admin/store/ratelimitpolicy"
	tokenquotapolicystore "github.com/lgc202/ingate/internal/admin/store/tokenquotapolicy"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Usage 表示一个目标当前被哪条策略应用
type Usage struct {
	DisplayName string
}

// UsageFinder 查询 Gateway 或 Route 是否仍被策略引用
type UsageFinder struct {
	rateLimitPolicies     *ratelimitpolicystore.Store
	accessControlPolicies *accesscontrolpolicystore.Store
	tokenQuotaPolicies    *tokenquotapolicystore.Store
}

// NewUsageFinder 创建策略目标引用查询器
func NewUsageFinder(
	rateLimitPolicies *ratelimitpolicystore.Store,
	accessControlPolicies *accesscontrolpolicystore.Store,
	tokenQuotaPolicies *tokenquotapolicystore.Store,
) *UsageFinder {
	return &UsageFinder{
		rateLimitPolicies:     rateLimitPolicies,
		accessControlPolicies: accessControlPolicies,
		tokenQuotaPolicies:    tokenQuotaPolicies,
	}
}

// Find 返回第一条仍引用目标的策略，没有引用时返回 nil
func (f *UsageFinder) Find(ctx context.Context, target resource.PolicyTargetRef) (*Usage, error) {
	rateLimitPolicies, err := f.rateLimitPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, policy := range rateLimitPolicies.Items {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			return &Usage{DisplayName: policy.Spec.DisplayName}, nil
		}
	}

	accessControlPolicies, err := f.accessControlPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, policy := range accessControlPolicies.Items {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			return &Usage{DisplayName: policy.Spec.DisplayName}, nil
		}
	}

	tokenQuotaPolicies, err := f.tokenQuotaPolicies.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, policy := range tokenQuotaPolicies.Items {
		if slices.Contains(policy.Spec.TargetRefs, target) {
			return &Usage{DisplayName: policy.Spec.DisplayName}, nil
		}
	}
	return nil, nil
}
