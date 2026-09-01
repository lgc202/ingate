package aiextproc

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	aiextprocv1 "github.com/lgc202/ingate/api/aiextproc/v1"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/tokenquotaconfig"
)

type tokenQuotaUsageKey struct {
	policyID string
	period   tokenquotabiz.Period
}

// Current 查询调用方当前实际命中的全部额度。
func (c *Client) Current(ctx context.Context, callerID string) ([]tokenquotabiz.Usage, error) {
	response, err := c.usage.GetCallerUsage(ctx, &aiextprocv1.GetCallerUsageRequest{CallerId: callerID})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		switch status.Code(err) {
		case codes.Unavailable, codes.DeadlineExceeded:
			return nil, errors.ServiceUnavailable(
				adminv1.ErrorReason_DEPENDENCY_UNAVAILABLE.String(),
				"实时额度暂时不可用，请稍后重试",
			).WithCause(err)
		default:
			return nil, fmt.Errorf("query caller token quota usage: %w", err)
		}
	}
	if response == nil {
		return nil, fmt.Errorf("AI ExtProc returned an empty token quota response for caller %q", callerID)
	}
	items := response.GetUsages()
	if len(items) > tokenquotaconfig.MaxPoliciesPerCaller*tokenquotaconfig.MaxLimits {
		return nil, fmt.Errorf("AI ExtProc returned too many token quota usages for caller %q", callerID)
	}
	usages := make([]tokenquotabiz.Usage, len(items))
	seen := make(map[tokenQuotaUsageKey]bool, len(items))
	for i, item := range items {
		usage, key, err := tokenQuotaUsageFromAPI(item, i)
		if err != nil {
			return nil, err
		}
		if seen[key] {
			return nil, fmt.Errorf(
				"AI ExtProc returned duplicate usage for policy %q and period %q",
				key.policyID,
				key.period,
			)
		}
		seen[key] = true
		usages[i] = usage
	}
	return usages, nil
}

func tokenQuotaUsageFromAPI(
	usage *aiextprocv1.TokenQuotaUsage,
	index int,
) (tokenquotabiz.Usage, tokenQuotaUsageKey, error) {
	if usage == nil {
		return tokenquotabiz.Usage{}, tokenQuotaUsageKey{}, fmt.Errorf(
			"AI ExtProc returned an empty token quota usage at index %d",
			index,
		)
	}
	policyID := usage.GetPolicyId()
	if !resourceconfig.IsCanonicalID(policyID) {
		return tokenquotabiz.Usage{}, tokenQuotaUsageKey{}, fmt.Errorf(
			"AI ExtProc returned an invalid policy ID at index %d",
			index,
		)
	}
	if !resourceconfig.IsValidDisplayName(usage.GetPolicyName()) {
		return tokenquotabiz.Usage{}, tokenQuotaUsageKey{}, fmt.Errorf(
			"AI ExtProc returned an invalid policy name at index %d",
			index,
		)
	}
	period, err := tokenQuotaPeriod(usage.GetPeriod())
	if err != nil {
		return tokenquotabiz.Usage{}, tokenQuotaUsageKey{}, fmt.Errorf(
			"AI ExtProc returned an invalid period at index %d: %w",
			index,
			err,
		)
	}
	if usage.GetUsedTokens() < 0 {
		return tokenquotabiz.Usage{}, tokenQuotaUsageKey{}, fmt.Errorf(
			"AI ExtProc returned negative token usage at index %d",
			index,
		)
	}
	if !tokenquotaconfig.IsValidTokenLimit(usage.GetLimitTokens()) {
		return tokenquotabiz.Usage{}, tokenQuotaUsageKey{}, fmt.Errorf(
			"AI ExtProc returned an invalid token limit at index %d",
			index,
		)
	}
	startedAt := usage.GetStartedAt()
	if startedAt == nil || startedAt.CheckValid() != nil {
		return tokenquotabiz.Usage{}, tokenQuotaUsageKey{}, fmt.Errorf(
			"AI ExtProc returned an invalid start time at index %d",
			index,
		)
	}
	resetsAt := usage.GetResetsAt()
	if resetsAt == nil || resetsAt.CheckValid() != nil {
		return tokenquotabiz.Usage{}, tokenQuotaUsageKey{}, fmt.Errorf(
			"AI ExtProc returned an invalid reset time at index %d",
			index,
		)
	}
	start := startedAt.AsTime()
	resetAt := resetsAt.AsTime()
	if !resetAt.After(start) {
		return tokenquotabiz.Usage{}, tokenQuotaUsageKey{}, fmt.Errorf(
			"AI ExtProc returned an invalid period range at index %d",
			index,
		)
	}

	key := tokenQuotaUsageKey{policyID: policyID, period: period}
	return tokenquotabiz.Usage{
		PolicyID:   policyID,
		PolicyName: usage.GetPolicyName(),
		Period:     period,
		Used:       usage.GetUsedTokens(),
		Limit:      usage.GetLimitTokens(),
		StartedAt:  start,
		ResetAt:    resetAt,
	}, key, nil
}

func tokenQuotaPeriod(period aiextprocv1.TokenQuotaPeriod) (tokenquotabiz.Period, error) {
	switch period {
	case aiextprocv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_DAY:
		return tokenquotabiz.PeriodDay, nil
	case aiextprocv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_WEEK:
		return tokenquotabiz.PeriodWeek, nil
	case aiextprocv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_MONTH:
		return tokenquotabiz.PeriodMonth, nil
	default:
		return "", fmt.Errorf("unsupported token quota period %q", period.String())
	}
}
