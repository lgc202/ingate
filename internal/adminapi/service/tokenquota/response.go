package tokenquota

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func tokenQuotaPolicyResponse(
	policy *resource.TokenQuotaPolicy,
	names biz.PolicyTargetNames,
) *adminv1.TokenQuotaPolicy {
	state, message := tokenQuotaPolicyState(policy)
	limits := make([]*adminv1.TokenQuotaLimit, len(policy.Spec.Limits))
	for i, limit := range policy.Spec.Limits {
		limits[i] = &adminv1.TokenQuotaLimit{
			Period: tokenQuotaPeriodResponse(limit.Period),
			Tokens: limit.Tokens,
		}
	}
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
	if !policy.Spec.Enabled {
		return adminv1.ResourceState_DISABLED, "策略已停用"
	}
	if len(policy.Spec.TargetRefs) == 0 {
		return adminv1.ResourceState_READY, "策略尚未应用到调用方"
	}
	// Token 额度由 AI ExtProc 直接监听并执行，不经过 Controller 的 Envoy 配置发布链路。
	return adminv1.ResourceState_READY, "策略已启用"
}

func tokenQuotaPolicyTargets(
	policy *resource.TokenQuotaPolicy,
	names biz.PolicyTargetNames,
) []*adminv1.PolicyTarget {
	state, message := tokenQuotaPolicyState(policy)
	targets := make([]*adminv1.PolicyTarget, len(policy.Spec.TargetRefs))
	for i, ref := range policy.Spec.TargetRefs {
		targets[i] = &adminv1.PolicyTarget{
			Kind:    adminv1.PolicyTargetKind_POLICY_TARGET_KIND_CALLER,
			Id:      ref.Name,
			Name:    names.Name(ref),
			State:   state,
			Message: message,
		}
	}
	return targets
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
	responses := make([]*adminv1.CallerTokenQuotaUsage, len(usages))
	for i, usage := range usages {
		responses[i] = &adminv1.CallerTokenQuotaUsage{
			PolicyId:        usage.PolicyID,
			PolicyName:      usage.PolicyName,
			Period:          tokenQuotaUsagePeriodResponse(usage.Period),
			UsedTokens:      usage.Used,
			LimitTokens:     usage.Limit,
			RemainingTokens: max(0, usage.Limit-usage.Used),
			StartedAt:       adminservice.Timestamp(usage.StartedAt),
			ResetsAt:        adminservice.Timestamp(usage.ResetAt),
		}
	}
	return &adminv1.GetCallerTokenQuotaUsageResponse{Usages: responses}
}

func tokenQuotaUsagePeriodResponse(period tokenquotabiz.Period) adminv1.TokenQuotaPeriod {
	switch period {
	case tokenquotabiz.PeriodDay:
		return adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_DAY
	case tokenquotabiz.PeriodWeek:
		return adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_WEEK
	case tokenquotabiz.PeriodMonth:
		return adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_MONTH
	default:
		return adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_UNSPECIFIED
	}
}
