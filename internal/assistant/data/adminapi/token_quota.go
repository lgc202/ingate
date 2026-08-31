package adminapi

import (
	"context"
	"errors"
	"fmt"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
)

// GetCallerTokenQuota 查询调用方身份和当前实际执行的额度用量。
// 两个 Admin API 结果在此处组合，Agent 业务层无需了解调用方与策略服务的协议边界。
func (c *Client) GetCallerTokenQuota(
	ctx context.Context,
	callerID string,
) (agenttool.CallerTokenQuota, error) {
	caller, err := c.callers.GetCaller(ctx, &adminv1.GetCallerRequest{Id: callerID})
	if err != nil {
		return agenttool.CallerTokenQuota{}, queryTargetError(
			fmt.Sprintf("get caller %s from Admin API", callerID),
			err,
		)
	}
	if caller == nil || caller.GetId() != callerID || caller.GetName() == "" {
		return agenttool.CallerTokenQuota{}, errors.New("invalid caller returned by Admin API")
	}

	result, err := c.tokenQuota.GetCallerTokenQuotaUsage(
		ctx,
		&adminv1.GetCallerTokenQuotaUsageRequest{CallerId: callerID},
	)
	if err != nil {
		return agenttool.CallerTokenQuota{}, queryTargetError(
			fmt.Sprintf("get caller %s token quota usage from Admin API", callerID),
			err,
		)
	}
	if result == nil {
		return agenttool.CallerTokenQuota{}, errors.New(
			"get caller token quota usage from Admin API: empty response",
		)
	}

	usages := make([]agenttool.TokenQuotaUsage, 0, len(result.GetUsages()))
	for _, usage := range result.GetUsages() {
		if err := validateTokenQuotaUsageResponse(usage); err != nil {
			return agenttool.CallerTokenQuota{}, err
		}
		usages = append(usages, tokenQuotaUsageFromAPI(usage))
	}
	return agenttool.CallerTokenQuota{
		CallerID:   caller.GetId(),
		CallerName: caller.GetName(),
		Enabled:    caller.GetEnabled(),
		Usages:     usages,
	}, nil
}

func tokenQuotaUsageFromAPI(usage *adminv1.CallerTokenQuotaUsage) agenttool.TokenQuotaUsage {
	return agenttool.TokenQuotaUsage{
		PolicyID:        usage.GetPolicyId(),
		PolicyName:      usage.GetPolicyName(),
		Period:          tokenQuotaPeriodFromAPI(usage.GetPeriod()),
		UsedTokens:      usage.GetUsedTokens(),
		LimitTokens:     usage.GetLimitTokens(),
		RemainingTokens: usage.GetRemainingTokens(),
		StartedAt:       protoTime(usage.GetStartedAt()),
		ResetsAt:        protoTime(usage.GetResetsAt()),
	}
}

func tokenQuotaPeriodFromAPI(period adminv1.TokenQuotaPeriod) string {
	switch period {
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_DAY:
		return "day"
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_WEEK:
		return "week"
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_MONTH:
		return "month"
	default:
		return "unknown"
	}
}

func validateTokenQuotaUsageResponse(usage *adminv1.CallerTokenQuotaUsage) error {
	if usage == nil || !validResourceID(usage.GetPolicyId()) || usage.GetPolicyName() == "" ||
		usage.GetUsedTokens() < 0 || usage.GetLimitTokens() <= 0 ||
		usage.GetRemainingTokens() < 0 || usage.GetRemainingTokens() > usage.GetLimitTokens() ||
		!validTimestamp(usage.GetStartedAt()) ||
		!validTimestamp(usage.GetResetsAt()) ||
		!usage.GetStartedAt().AsTime().Before(usage.GetResetsAt().AsTime()) {
		return errors.New("invalid caller token quota usage returned by Admin API")
	}
	switch usage.GetPeriod() {
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_DAY,
		adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_WEEK,
		adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_MONTH:
		return nil
	default:
		return errors.New("caller token quota usage has an invalid period")
	}
}
