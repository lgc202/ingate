// Package ratelimit 处理请求限流策略的业务规则和资源协作。
package ratelimit

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义请求限流策略管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.RateLimitPolicy], error)
	Get(ctx context.Context, policyID string) (*resource.RateLimitPolicy, error)
	Create(
		ctx context.Context,
		policyID string,
		spec resource.RateLimitPolicySpec,
	) (*resource.RateLimitPolicy, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.RateLimitPolicy,
		spec resource.RateLimitPolicySpec,
	) (*resource.RateLimitPolicy, error)
	Delete(ctx context.Context, observed *resource.RateLimitPolicy) error
}

// Usecase 提供请求限流策略管理能力。
type Usecase struct {
	policies *biz.PolicyUsecase[resource.RateLimitPolicy, resource.RateLimitPolicySpec]
}

// NewUsecase 创建请求限流策略用例。
func NewUsecase(
	store Store,
	gateways biz.GatewayReader,
	routes biz.RouteReader,
) *Usecase {
	return &Usecase{
		policies: biz.NewPolicyUsecase(
			store,
			biz.NewPolicyTargetResolver(gateways, routes),
			policyAttributes,
			policyTargetRefs,
		),
	}
}

// List 返回满足筛选条件的请求限流策略。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (biz.PolicyPage[resource.RateLimitPolicy], error) {
	return uc.policies.List(ctx, page, filter)
}

// Get 返回指定请求限流策略。
func (uc *Usecase) Get(
	ctx context.Context,
	policyID string,
) (biz.PolicyView[resource.RateLimitPolicy], error) {
	return uc.policies.Get(ctx, policyID)
}

// Create 创建请求限流策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.RateLimitPolicySpec,
) (biz.PolicyView[resource.RateLimitPolicy], error) {
	return uc.policies.Create(ctx, spec)
}

// Replace 使用配置版本完整替换请求限流策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.RateLimitPolicySpec,
) (biz.PolicyView[resource.RateLimitPolicy], error) {
	return uc.policies.Replace(ctx, policyID, expectedGeneration, spec)
}

// Delete 使用配置版本删除请求限流策略。
func (uc *Usecase) Delete(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
) error {
	return uc.policies.Delete(ctx, policyID, expectedGeneration)
}

func policyAttributes(policy *resource.RateLimitPolicy) biz.PolicyAttributes {
	return biz.PolicyAttributes{
		Generation:  policy.Generation,
		DisplayName: policy.Spec.DisplayName,
		Enabled:     policy.Spec.Enabled,
		TargetRefs:  policy.Spec.TargetRefs,
		Status: biz.PolicyStatus(
			policy.Generation,
			policy.Spec.Enabled,
			len(policy.Spec.TargetRefs),
			policy.Status.Conditions,
		),
	}
}

func policyTargetRefs(spec resource.RateLimitPolicySpec) []resource.PolicyTargetRef {
	return spec.TargetRefs
}
