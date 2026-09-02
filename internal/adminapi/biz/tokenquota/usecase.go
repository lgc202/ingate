// Package tokenquota 处理模型 Token 额度的业务规则和资源协作。
package tokenquota

import (
	"context"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义 Token 额度策略管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page pagination.Request) (pagination.Result[resource.TokenQuotaPolicy], error)
	Get(ctx context.Context, policyID string) (*resource.TokenQuotaPolicy, error)
	Create(ctx context.Context, policyID string, spec resource.TokenQuotaPolicySpec) (*resource.TokenQuotaPolicy, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.TokenQuotaPolicy,
		spec resource.TokenQuotaPolicySpec,
	) (*resource.TokenQuotaPolicy, error)
	Delete(ctx context.Context, observed *resource.TokenQuotaPolicy) error
}

// CallerReader 定义额度目标展示和实时用量查询所需的 Caller 读取能力。
type CallerReader interface {
	Get(ctx context.Context, callerID string) (*resource.Caller, error)
	ListByIDs(ctx context.Context, callerIDs []string) (map[string]*resource.Caller, error)
}

// UsageReader 定义 Admin API 查询 AI ExtProc 实时额度所需的能力。
type UsageReader interface {
	Current(ctx context.Context, callerID string) ([]Usage, error)
}

// Usecase 提供 Token 额度策略管理和实时用量查询。
type Usecase struct {
	store   Store
	targets *policy.PolicyTargetResolver
	callers CallerReader
	usage   UsageReader
}

// NewUsecase 创建 Token 额度策略用例。
func NewUsecase(store Store, callers CallerReader, usage UsageReader) *Usecase {
	return &Usecase{
		store:   store,
		targets: policy.NewCallerPolicyTargetResolver(callers),
		callers: callers,
		usage:   usage,
	}
}

// PolicyStatus 返回 Token 额度策略当前对调用方流量的执行状态。
// 额度由 AI ExtProc 直接监听执行，不依赖 Controller 的配置发布 Conditions。
func PolicyStatus(item *resource.TokenQuotaPolicy) resourceview.Status {
	if !item.Spec.Enabled {
		return resourceview.Status{State: resourceview.StateDisabled, Reason: resourceview.ReasonDisabled}
	}
	if len(item.Spec.TargetRefs) == 0 {
		return resourceview.Status{State: resourceview.StateReady, Reason: resourceview.ReasonUnapplied}
	}
	return resourceview.Status{State: resourceview.StateReady, Reason: resourceview.ReasonReady}
}

// List 返回满足筛选条件的 Token 额度策略。
func (uc *Usecase) List(
	ctx context.Context,
	page pagination.Request,
	filter resourceview.Filter,
) (policy.Page[resource.TokenQuotaPolicy], error) {
	result, err := resourceview.FilterPage(ctx, page, uc.store.ListPage, func(item resource.TokenQuotaPolicy) bool {
		return filter.Match(item.Spec.DisplayName, item.Spec.Enabled, PolicyStatus(&item))
	})
	if err != nil {
		return policy.Page[resource.TokenQuotaPolicy]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, collectTargetRefs(result.Items))
	if err != nil {
		return policy.Page[resource.TokenQuotaPolicy]{}, err
	}
	return policy.Page[resource.TokenQuotaPolicy]{
		Items: result.Items, TargetNames: targetNames, NextCursor: result.NextCursor,
	}, nil
}

// Get 返回指定 Token 额度策略。
func (uc *Usecase) Get(ctx context.Context, policyID string) (policy.View[resource.TokenQuotaPolicy], error) {
	item, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return policy.View[resource.TokenQuotaPolicy]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, item.Spec.TargetRefs)
	if err != nil {
		return policy.View[resource.TokenQuotaPolicy]{}, err
	}
	return policy.View[resource.TokenQuotaPolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Create 创建 Token 额度策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.TokenQuotaPolicySpec,
) (policy.View[resource.TokenQuotaPolicy], error) {
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return policy.View[resource.TokenQuotaPolicy]{}, err
	}
	item, err := uc.store.Create(ctx, uuid.NewString(), spec)
	if err != nil {
		return policy.View[resource.TokenQuotaPolicy]{}, err
	}
	return policy.View[resource.TokenQuotaPolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Replace 使用配置版本完整替换 Token 额度策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.TokenQuotaPolicySpec,
) (policy.View[resource.TokenQuotaPolicy], error) {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return policy.View[resource.TokenQuotaPolicy]{}, err
	}
	if current.Generation != expectedGeneration {
		return policy.View[resource.TokenQuotaPolicy]{}, apperror.ResourceVersionConflict()
	}
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return policy.View[resource.TokenQuotaPolicy]{}, err
	}
	item, err := uc.store.ReplaceSpec(ctx, current, spec)
	if err != nil {
		return policy.View[resource.TokenQuotaPolicy]{}, err
	}
	return policy.View[resource.TokenQuotaPolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Delete 使用配置版本删除 Token 额度策略。
func (uc *Usecase) Delete(ctx context.Context, policyID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if current.Generation != expectedGeneration {
		return apperror.ResourceVersionConflict()
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

func collectTargetRefs(items []resource.TokenQuotaPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for i := range items {
		refs = append(refs, items[i].Spec.TargetRefs...)
	}
	return refs
}
