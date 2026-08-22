package aiextproc

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	aiextprocv1 "github.com/lgc202/ingate/api/aiextproc/v1"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
)

// TokenQuotaUsageReader 从 AI ExtProc 读取实时额度计数
type TokenQuotaUsageReader struct {
	client *Client
}

// NewTokenQuotaUsageReader 创建实时额度读取器
func NewTokenQuotaUsageReader(client *Client) *TokenQuotaUsageReader {
	return &TokenQuotaUsageReader{client: client}
}

// Current 查询调用方当前实际命中的全部额度
func (r *TokenQuotaUsageReader) Current(ctx context.Context, callerID string) ([]tokenquotabiz.Usage, error) {
	response, err := r.client.usage.GetCallerUsage(ctx, &aiextprocv1.GetCallerUsageRequest{CallerId: callerID})
	if err != nil {
		switch status.Code(err) {
		case codes.Unavailable, codes.DeadlineExceeded:
			return nil, tokenquotabiz.UsageUnavailable(err)
		default:
			return nil, fmt.Errorf("query caller token quota usage: %w", err)
		}
	}
	usages := make([]tokenquotabiz.Usage, 0, len(response.GetUsages()))
	for _, usage := range response.GetUsages() {
		period, err := tokenQuotaPeriod(usage.GetPeriod())
		if err != nil {
			return nil, err
		}
		usages = append(usages, tokenquotabiz.Usage{
			PolicyID:   usage.GetPolicyId(),
			PolicyName: usage.GetPolicyName(),
			Period:     period,
			Used:       usage.GetUsedTokens(),
			Limit:      usage.GetLimitTokens(),
			Start:      usage.GetStartedAt().AsTime(),
			ResetAt:    usage.GetResetsAt().AsTime(),
		})
	}
	return usages, nil
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
