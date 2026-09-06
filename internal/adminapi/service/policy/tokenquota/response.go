package tokenquota

import (
	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/policy/tokenquota"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	"github.com/lgc202/ingate/internal/adminapi/service/conversion"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func policyResponse(
	policy *resource.TokenQuotaPolicy,
	names policy.TargetNames,
) *adminv1.TokenQuotaPolicy {
	state, message := policyState(policy)
	limits := lo.Map(policy.Spec.Limits, func(limit resource.TokenQuotaLimit, _ int) *adminv1.TokenQuotaLimit {
		return &adminv1.TokenQuotaLimit{
			Period: periodResponse(limit.Period),
			Tokens: limit.Tokens,
		}
	})
	return &adminv1.TokenQuotaPolicy{
		Id:        policy.Name,
		Name:      policy.Spec.DisplayName,
		Enabled:   policy.Spec.Enabled,
		Targets:   policyTargets(policy, names),
		TimeZone:  policy.Spec.TimeZone,
		Limits:    limits,
		State:     state,
		Message:   message,
		Version:   policy.Generation,
		CreatedAt: conversion.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt: conversion.Timestamp(conversion.ResourceUpdatedAt(policy.Annotations)),
	}
}

func policyState(
	policy *resource.TokenQuotaPolicy,
) (adminv1.ResourceState, string) {
	resourceStatus := tokenquotabiz.PolicyStatus(policy)
	message := "策略已启用"
	switch resourceStatus.Reason {
	case status.ReasonDisabled:
		message = "策略已停用"
	case status.ReasonUnapplied:
		message = "策略尚未应用到调用方"
	}
	return conversion.ResourceState(resourceStatus.State), message
}

func policyTargets(
	policy *resource.TokenQuotaPolicy,
	names policy.TargetNames,
) []*adminv1.PolicyTarget {
	state, message := policyState(policy)
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

func periodResponse(period resource.TokenQuotaPeriod) adminv1.TokenQuotaPeriod {
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

func usageResponse(
	usages []tokenquotabiz.Usage,
) *adminv1.GetCallerTokenQuotaUsageResponse {
	return &adminv1.GetCallerTokenQuotaUsageResponse{
		Usages: lo.Map(usages, func(usage tokenquotabiz.Usage, _ int) *adminv1.CallerTokenQuotaUsage {
			return &adminv1.CallerTokenQuotaUsage{
				PolicyId:        usage.PolicyID,
				PolicyName:      usage.PolicyName,
				Period:          periodResponse(usage.Period),
				UsedTokens:      usage.Used,
				LimitTokens:     usage.Limit,
				RemainingTokens: max(0, usage.Limit-usage.Used),
				StartedAt:       conversion.Timestamp(usage.StartedAt),
				ResetsAt:        conversion.Timestamp(usage.ResetAt),
			}
		}),
	}
}
