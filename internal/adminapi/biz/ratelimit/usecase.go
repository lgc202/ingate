// Package ratelimit 处理请求限流策略的业务规则和资源协作。
package ratelimit

import (
	"context"

	"github.com/google/uuid"

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

// PolicyPage 保存一页请求限流策略及其目标展示名称。
type PolicyPage struct {
	Items       []resource.RateLimitPolicy
	TargetNames biz.PolicyTargetNames
	NextCursor  string
}

// PolicyView 保存单条请求限流策略及其目标展示名称。
type PolicyView struct {
	Policy      *resource.RateLimitPolicy
	TargetNames biz.PolicyTargetNames
}

// Usecase 协调请求限流策略的目标校验和持久化。
type Usecase struct {
	store   Store
	targets *biz.PolicyTargetResolver
}

// NewUsecase 创建请求限流策略用例。
func NewUsecase(
	store Store,
	gateways biz.GatewayLister,
	routes biz.RouteLister,
) *Usecase {
	return &Usecase{
		store:   store,
		targets: biz.NewPolicyTargetResolver(gateways, routes),
	}
}

// List 返回满足筛选条件的请求限流策略。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (PolicyPage, error) {
	result, err := biz.FilterPage(
		ctx,
		page,
		uc.store.ListPage,
		func(policy resource.RateLimitPolicy) bool {
			status := biz.PolicyStatus(
				policy.Generation,
				policy.Spec.Enabled,
				len(policy.Spec.TargetRefs),
				policy.Status.Conditions,
			)
			return filter.Match(policy.Spec.DisplayName, policy.Spec.Enabled, status)
		},
	)
	if err != nil {
		return PolicyPage{}, err
	}

	targetNames, err := uc.targets.DisplayNames(ctx, collectTargetRefs(result.Items))
	if err != nil {
		return PolicyPage{}, err
	}
	return PolicyPage{
		Items:       result.Items,
		TargetNames: targetNames,
		NextCursor:  result.NextCursor,
	}, nil
}

// Get 返回指定请求限流策略。
func (uc *Usecase) Get(ctx context.Context, policyID string) (PolicyView, error) {
	policy, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return PolicyView{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return PolicyView{}, err
	}
	return PolicyView{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建请求限流策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.RateLimitPolicySpec,
) (PolicyView, error) {
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return PolicyView{}, err
	}

	policyID := uuid.NewString()
	policy, err := uc.store.Create(ctx, policyID, spec)
	if err != nil {
		return PolicyView{}, err
	}
	return PolicyView{Policy: policy, TargetNames: targetNames}, nil
}

// Replace 使用配置版本完整替换请求限流策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.RateLimitPolicySpec,
) (PolicyView, error) {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return PolicyView{}, err
	}

	if current.Generation != expectedGeneration {
		return PolicyView{}, biz.ErrResourceVersionConflict
	}
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return PolicyView{}, err
	}

	policy, err := uc.store.ReplaceSpec(ctx, current, spec)
	if err != nil {
		return PolicyView{}, err
	}
	return PolicyView{Policy: policy, TargetNames: targetNames}, nil
}

// Delete 删除请求限流策略。
func (uc *Usecase) Delete(ctx context.Context, policyID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if current.Generation != expectedGeneration {
		return biz.ErrResourceVersionConflict
	}
	return uc.store.Delete(ctx, current)
}

func collectTargetRefs(policies []resource.RateLimitPolicy) []resource.PolicyTargetRef {
	targetCount := 0
	for i := range policies {
		targetCount += len(policies[i].Spec.TargetRefs)
	}

	refs := make([]resource.PolicyTargetRef, 0, targetCount)
	for i := range policies {
		refs = append(refs, policies[i].Spec.TargetRefs...)
	}
	return refs
}
