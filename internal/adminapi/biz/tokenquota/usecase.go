// Package tokenquota 处理模型 Token 额度的业务规则和资源协作。
package tokenquota

import (
	"context"

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
	biz.CallerReader
	Get(ctx context.Context, callerID string) (*resource.Caller, error)
}

// UsageReader 定义 Admin API 查询 AI ExtProc 实时额度所需的能力。
type UsageReader interface {
	Current(ctx context.Context, callerID string) ([]Usage, error)
}

// Usecase 提供 Token 额度策略管理和实时用量查询。
type Usecase struct {
	policies *biz.PolicyUsecase[resource.TokenQuotaPolicy, resource.TokenQuotaPolicySpec]
	callers  CallerReader
	usage    UsageReader
}

// NewUsecase 创建 Token 额度策略用例。
func NewUsecase(store Store, callers CallerReader, usage UsageReader) *Usecase {
	return &Usecase{
		policies: biz.NewPolicyUsecase(
			store,
			biz.NewCallerPolicyTargetResolver(callers),
			policyAttributes,
			policyTargetRefs,
		),
		callers: callers,
		usage:   usage,
	}
}

// PolicyStatus 返回 Token 额度策略当前对调用方流量的执行状态。
// 额度由 AI ExtProc 直接监听执行，不依赖 Controller 的配置发布 Conditions。
func PolicyStatus(policy *resource.TokenQuotaPolicy) biz.ResourceStatus {
	if !policy.Spec.Enabled {
		return biz.ResourceStatus{State: biz.ResourceStateDisabled, Reason: biz.ReasonDisabled}
	}
	if len(policy.Spec.TargetRefs) == 0 {
		return biz.ResourceStatus{State: biz.ResourceStateReady, Reason: biz.ReasonUnapplied}
	}
	return biz.ResourceStatus{State: biz.ResourceStateReady, Reason: biz.ReasonReady}
}

// List 返回满足筛选条件的 Token 额度策略。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (biz.PolicyPage[resource.TokenQuotaPolicy], error) {
	return uc.policies.List(ctx, page, filter)
}

// Get 返回指定 Token 额度策略。
func (uc *Usecase) Get(
	ctx context.Context,
	policyID string,
) (biz.PolicyView[resource.TokenQuotaPolicy], error) {
	return uc.policies.Get(ctx, policyID)
}

// Create 创建 Token 额度策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.TokenQuotaPolicySpec,
) (biz.PolicyView[resource.TokenQuotaPolicy], error) {
	return uc.policies.Create(ctx, spec)
}

// Replace 使用配置版本完整替换 Token 额度策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.TokenQuotaPolicySpec,
) (biz.PolicyView[resource.TokenQuotaPolicy], error) {
	return uc.policies.Replace(ctx, policyID, expectedGeneration, spec)
}

// Delete 使用配置版本删除 Token 额度策略。
func (uc *Usecase) Delete(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
) error {
	return uc.policies.Delete(ctx, policyID, expectedGeneration)
}

// CurrentUsage 返回调用方当前实际执行的 Token 额度。
func (uc *Usecase) CurrentUsage(ctx context.Context, callerID string) ([]Usage, error) {
	if _, err := uc.callers.Get(ctx, callerID); err != nil {
		return nil, err
	}
	return uc.usage.Current(ctx, callerID)
}
func policyAttributes(policy *resource.TokenQuotaPolicy) biz.PolicyAttributes {
	return biz.PolicyAttributes{
		Generation:  policy.Generation,
		DisplayName: policy.Spec.DisplayName,
		Enabled:     policy.Spec.Enabled,
		TargetRefs:  policy.Spec.TargetRefs,
		Status:      PolicyStatus(policy),
	}
}

func policyTargetRefs(spec resource.TokenQuotaPolicySpec) []resource.PolicyTargetRef {
	return spec.TargetRefs
}
