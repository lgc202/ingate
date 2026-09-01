package aiextproc

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	aiextprocv1 "github.com/lgc202/ingate/api/aiextproc/v1"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/tokenquotaconfig"
)

type tokenQuotaUsageKey struct {
	policyID string
	period   resource.TokenQuotaPeriod
}

// Current 查询调用方当前实际命中的全部额度。
func (c *Client) Current(ctx context.Context, callerID string) ([]tokenquotabiz.Usage, error) {
	response, err := c.usage.GetCallerUsage(ctx, &aiextprocv1.GetCallerUsageRequest{CallerId: callerID})
	if err != nil {
		return nil, currentUsageError(ctx, err)
	}
	return decodeTokenQuotaUsages(callerID, response)
}

func currentUsageError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return tokenquotabiz.Unavailable(err)
	default:
		return fmt.Errorf("query caller token quota usage: %w", err)
	}
}

func decodeTokenQuotaUsages(
	callerID string,
	response *aiextprocv1.GetCallerUsageResponse,
) ([]tokenquotabiz.Usage, error) {
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
		usage, err := decodeTokenQuotaUsage(item, i)
		if err != nil {
			return nil, err
		}
		key := tokenQuotaUsageKey{policyID: usage.PolicyID, period: usage.Period}
		if seen[key] {
			return nil, fmt.Errorf(
				"AI ExtProc returned duplicate usage for policy %q and period %q",
				key.policyID,
				strings.ToLower(string(key.period)),
			)
		}
		seen[key] = true
		usages[i] = usage
	}
	return usages, nil
}

func decodeTokenQuotaUsage(
	item *aiextprocv1.TokenQuotaUsage,
	index int,
) (tokenquotabiz.Usage, error) {
	if item == nil {
		return tokenquotabiz.Usage{}, fmt.Errorf(
			"AI ExtProc returned an empty token quota usage at index %d",
			index,
		)
	}
	policyID := item.GetPolicyId()
	if !resourceconfig.IsCanonicalID(policyID) {
		return tokenquotabiz.Usage{}, fmt.Errorf(
			"AI ExtProc returned an invalid policy ID at index %d",
			index,
		)
	}
	if !resourceconfig.IsValidDisplayName(item.GetPolicyName()) {
		return tokenquotabiz.Usage{}, fmt.Errorf(
			"AI ExtProc returned an invalid policy name at index %d",
			index,
		)
	}
	period, err := tokenQuotaPeriod(item.GetPeriod())
	if err != nil {
		return tokenquotabiz.Usage{}, fmt.Errorf(
			"AI ExtProc returned an invalid period at index %d: %w",
			index,
			err,
		)
	}
	if item.GetUsedTokens() < 0 {
		return tokenquotabiz.Usage{}, fmt.Errorf(
			"AI ExtProc returned negative token usage at index %d",
			index,
		)
	}
	if !tokenquotaconfig.IsValidTokenLimit(item.GetLimitTokens()) {
		return tokenquotabiz.Usage{}, fmt.Errorf(
			"AI ExtProc returned an invalid token limit at index %d",
			index,
		)
	}
	startedAt := item.GetStartedAt()
	if startedAt == nil || startedAt.CheckValid() != nil {
		return tokenquotabiz.Usage{}, fmt.Errorf(
			"AI ExtProc returned an invalid start time at index %d",
			index,
		)
	}
	resetsAt := item.GetResetsAt()
	if resetsAt == nil || resetsAt.CheckValid() != nil {
		return tokenquotabiz.Usage{}, fmt.Errorf(
			"AI ExtProc returned an invalid reset time at index %d",
			index,
		)
	}
	start := startedAt.AsTime()
	resetAt := resetsAt.AsTime()
	if !resetAt.After(start) {
		return tokenquotabiz.Usage{}, fmt.Errorf(
			"AI ExtProc returned an invalid period range at index %d",
			index,
		)
	}

	return tokenquotabiz.Usage{
		PolicyID:   policyID,
		PolicyName: item.GetPolicyName(),
		Period:     period,
		Used:       item.GetUsedTokens(),
		Limit:      item.GetLimitTokens(),
		StartedAt:  start,
		ResetAt:    resetAt,
	}, nil
}

func tokenQuotaPeriod(period aiextprocv1.TokenQuotaPeriod) (resource.TokenQuotaPeriod, error) {
	switch period {
	case aiextprocv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_DAY:
		return resource.TokenQuotaPeriodDay, nil
	case aiextprocv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_WEEK:
		return resource.TokenQuotaPeriodWeek, nil
	case aiextprocv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_MONTH:
		return resource.TokenQuotaPeriodMonth, nil
	default:
		return "", fmt.Errorf("unsupported token quota period %q", period.String())
	}
}
