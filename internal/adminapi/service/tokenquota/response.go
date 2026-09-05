package tokenquota

import (
	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func tokenQuotaPolicyResponse(
	policy *resource.TokenQuotaPolicy,
	names policy.TargetNames,
) *adminv1.TokenQuotaPolicy {
	state, message := tokenQuotaPolicyState(policy)
	limits := lo.Map(policy.Spec.Limits, func(limit resource.TokenQuotaLimit, _ int) *adminv1.TokenQuotaLimit {
		return &adminv1.TokenQuotaLimit{
			Period: tokenQuotaPeriodResponse(limit.Period),
			Tokens: limit.Tokens,
		}
	})
	return &adminv1.TokenQuotaPolicy{
		Id:        policy.Name,
		Name:      policy.Spec.DisplayName,
		Enabled:   policy.Spec.Enabled,
		Targets:   tokenQuotaPolicyTargets(policy, names),
		TimeZone:  policy.Spec.TimeZone,
		Limits:    limits,
		State:     state,
		Message:   message,
		Version:   policy.Generation,
		CreatedAt: adminservice.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt: adminservice.Timestamp(adminservice.ResourceUpdatedAt(policy.Annotations)),
	}
}

func tokenQuotaPolicyState(
	policy *resource.TokenQuotaPolicy,
) (adminv1.ResourceState, string) {
	status := tokenquotabiz.PolicyStatus(policy)
	message := "策略已启用"
	switch status.Reason {
	case resourceview.ReasonDisabled:
		message = "策略已停用"
	case resourceview.ReasonUnapplied:
		message = "策略尚未应用到调用方"
	}
	return adminservice.ResourceState(status.State), message
}

func tokenQuotaPolicyTargets(
	policy *resource.TokenQuotaPolicy,
	names policy.TargetNames,
) []*adminv1.PolicyTarget {
	state, message := tokenQuotaPolicyState(policy)
	return lo.Map(policy.Spec.TargetRefs, func(ref resource.PolicyTargetRef, _ int) *adminv1.PolicyTarget {
		return &adminv1.PolicyTarget{
			Kind:    adminv1.PolicyTargetKind_POLICY_TARGET_KIND_CALLER,
			Id:      ref.Name,
			Name:    names.Name(ref),
			State:   state,
			Message: message,
		}
	})
}

func tokenQuotaPeriodResponse(period resource.TokenQuotaPeriod) adminv1.TokenQuotaPeriod {
	switch period {
	case resource.TokenQuotaPeriodDay:
		return adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_DAY
	case resource.TokenQuotaPeriodWeek:
		return adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_WEEK
	case resource.TokenQuotaPeriodMonth:
		return adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_MONTH
	default:
		return adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_UNSPECIFIED
	}
}

func tokenQuotaUsageResponse(
	usages []tokenquotabiz.Usage,
) *adminv1.GetCallerTokenQuotaUsageResponse {
	return &adminv1.GetCallerTokenQuotaUsageResponse{
		Usages: lo.Map(usages, func(usage tokenquotabiz.Usage, _ int) *adminv1.CallerTokenQuotaUsage {
			return &adminv1.CallerTokenQuotaUsage{
				PolicyId:        usage.PolicyID,
				PolicyName:      usage.PolicyName,
				Period:          tokenQuotaPeriodResponse(usage.Period),
				UsedTokens:      usage.Used,
				LimitTokens:     usage.Limit,
				RemainingTokens: max(0, usage.Limit-usage.Used),
				StartedAt:       adminservice.Timestamp(usage.StartedAt),
				ResetsAt:        adminservice.Timestamp(usage.ResetAt),
			}
		}),
	}
}
