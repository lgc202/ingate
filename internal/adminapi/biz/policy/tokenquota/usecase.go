// Package tokenquota 处理模型 Token 额度的业务规则和资源协作。
package tokenquota

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义 Token 额度策略管理所需的持久化能力。
type Store interface {
	policy.Store[resource.TokenQuotaPolicy, resource.TokenQuotaPolicySpec]
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
	*policy.Usecase[resource.TokenQuotaPolicy, resource.TokenQuotaPolicySpec]
	callers CallerReader
	usage   UsageReader
}

// NewUsecase 创建 Token 额度策略用例。
func NewUsecase(store Store, callers CallerReader, usage UsageReader) *Usecase {
	return &Usecase{
		Usecase: policy.NewUsecase(
			store,
			policy.NewCallerTargetResolver(callers),
			policy.ResourceAccessors[resource.TokenQuotaPolicy, resource.TokenQuotaPolicySpec]{
				DisplayName: func(item *resource.TokenQuotaPolicy) string { return item.Spec.DisplayName },
				Enabled:     func(item *resource.TokenQuotaPolicy) bool { return item.Spec.Enabled },
				Generation:  func(item *resource.TokenQuotaPolicy) int64 { return item.Generation },
				TargetRefs:  func(item *resource.TokenQuotaPolicy) []resource.PolicyTargetRef { return item.Spec.TargetRefs },
				TargetRefsFromSpec: func(spec resource.TokenQuotaPolicySpec) []resource.PolicyTargetRef {
					return spec.TargetRefs
				},
				Status: PolicyStatus,
			},
			nil,
		),
		callers: callers,
		usage:   usage,
	}
}

// PolicyStatus 返回 Token 额度策略当前对调用方流量的执行状态。
// 额度由 AI ExtProc 直接监听执行，不依赖 Controller 的配置发布 Conditions。
func PolicyStatus(item *resource.TokenQuotaPolicy) status.Status {
	if !item.Spec.Enabled {
		return status.Status{State: status.StateDisabled, Reason: status.ReasonDisabled}
	}
	if len(item.Spec.TargetRefs) == 0 {
		return status.Status{State: status.StateReady, Reason: status.ReasonUnapplied}
	}
	return status.Status{State: status.StateReady, Reason: status.ReasonReady}
}

// CurrentUsage 返回调用方当前实际执行的 Token 额度。
func (uc *Usecase) CurrentUsage(ctx context.Context, callerID string) ([]Usage, error) {
	if _, err := uc.callers.Get(ctx, callerID); err != nil {
		return nil, err
	}
	return uc.usage.Current(ctx, callerID)
}
