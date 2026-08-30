// Package tokenquota 处理模型 Token 额度的业务规则和资源协作。
package tokenquota

import (
	"context"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义 Token 额度策略管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.TokenQuotaPolicy], error)
	Get(ctx context.Context, policyID string) (*resource.TokenQuotaPolicy, error)
	Create(
		ctx context.Context,
		policyID string,
		spec resource.TokenQuotaPolicySpec,
	) (*resource.TokenQuotaPolicy, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.TokenQuotaPolicy,
		spec resource.TokenQuotaPolicySpec,
	) (*resource.TokenQuotaPolicy, error)
	Delete(ctx context.Context, observed *resource.TokenQuotaPolicy) error
}

// CallerReader 定义额度目标展示和实时用量查询所需的 Caller 读取能力。
type CallerReader interface {
	biz.CallerLister
	Get(ctx context.Context, callerID string) (*resource.Caller, error)
}

// UsageReader 定义 Admin API 查询 AI ExtProc 实时额度所需的能力。
type UsageReader interface {
	Current(ctx context.Context, callerID string) ([]Usage, error)
}

// PolicyPage 保存一页 Token 额度策略及其调用方展示名称。
type PolicyPage struct {
	Items       []resource.TokenQuotaPolicy
	TargetNames biz.PolicyTargetNames
	NextCursor  string
}

// PolicyView 保存单条 Token 额度策略及其调用方展示名称。
type PolicyView struct {
	Policy      *resource.TokenQuotaPolicy
	TargetNames biz.PolicyTargetNames
}

// Usecase 协调 Token 额度策略的调用方校验、持久化和实时用量查询。
type Usecase struct {
	store   Store
	callers CallerReader
	usage   UsageReader
	targets *biz.PolicyTargetResolver
}

// NewUsecase 创建 Token 额度策略用例。
func NewUsecase(store Store, callers CallerReader, usage UsageReader) *Usecase {
	return &Usecase{
		store:   store,
		callers: callers,
		usage:   usage,
		targets: biz.NewCallerPolicyTargetResolver(callers),
	}
}

// List 返回满足筛选条件的 Token 额度策略。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (PolicyPage, error) {
	result, err := biz.FilterPage(
		ctx,
		page,
		uc.store.ListPage,
		func(policy resource.TokenQuotaPolicy) bool {
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

// Get 返回指定 Token 额度策略。
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

// Create 创建 Token 额度策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.TokenQuotaPolicySpec,
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

// Replace 使用配置版本完整替换 Token 额度策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.TokenQuotaPolicySpec,
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

// Delete 删除 Token 额度策略。
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

// CurrentUsage 返回调用方当前实际执行的 Token 额度。
func (uc *Usecase) CurrentUsage(ctx context.Context, callerID string) ([]Usage, error) {
	if _, err := uc.callers.Get(ctx, callerID); err != nil {
		return nil, err
	}
	return uc.usage.Current(ctx, callerID)
}

func collectTargetRefs(policies []resource.TokenQuotaPolicy) []resource.PolicyTargetRef {
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
