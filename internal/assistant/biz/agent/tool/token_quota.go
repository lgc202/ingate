package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/google/uuid"
)

type callerTokenQuotaInput struct {
	CallerID string `json:"caller_id" jsonschema_description:"get_request_record 返回的调用方 ID"`
}

type callerTokenQuotaOutput struct {
	Summary string                `json:"summary"`
	Source  string                `json:"source,omitempty"`
	Status  string                `json:"status"`
	Caller  *callerTokenQuotaInfo `json:"caller,omitempty"`
}

type callerTokenQuotaInfo struct {
	ID      string                `json:"id"`
	Name    string                `json:"name"`
	Enabled bool                  `json:"enabled"`
	Limited bool                  `json:"limited"`
	Usages  []tokenQuotaUsageInfo `json:"usages"`
}

type tokenQuotaUsageInfo struct {
	PolicyID        string `json:"policy_id"`
	PolicyName      string `json:"policy_name"`
	Period          string `json:"period"`
	UsedTokens      int64  `json:"used_tokens"`
	LimitTokens     int64  `json:"limit_tokens"`
	RemainingTokens int64  `json:"remaining_tokens"`
	StartedAt       string `json:"started_at"`
	ResetsAt        string `json:"resets_at"`
}

func newCallerTokenQuotaTool(quotas CallerTokenQuotaReader) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		getCallerTokenQuotaTool,
		"查询一个调用方当前实际生效的 Token 额度、已用量和重置时间。仅在具体请求因 Token 额度被拒绝，或用户明确询问某个调用方额度时使用。",
		func(ctx context.Context, input callerTokenQuotaInput) (callerTokenQuotaOutput, error) {
			return getCallerTokenQuota(ctx, quotas, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", getCallerTokenQuotaTool, err)
	}
	return definition, nil
}

func getCallerTokenQuota(
	ctx context.Context,
	quotas CallerTokenQuotaReader,
	input callerTokenQuotaInput,
) (callerTokenQuotaOutput, error) {
	callerID := strings.TrimSpace(input.CallerID)
	if _, err := uuid.Parse(callerID); err != nil {
		return callerTokenQuotaInputResult(
			invalidInputf("caller_id must be a valid caller ID returned by get_request_record"),
		)
	}

	quota, err := quotas.GetCallerTokenQuota(ctx, callerID)
	if err != nil {
		return callerTokenQuotaOutput{}, err
	}
	info := callerTokenQuotaInfoFromQuota(quota)
	summary := fmt.Sprintf("调用方 %s 当前有 %d 个生效额度周期", quota.CallerName, len(quota.Usages))
	if len(quota.Usages) == 0 {
		summary = fmt.Sprintf("调用方 %s 当前没有生效的 Token 额度限制", quota.CallerName)
	}
	return callerTokenQuotaOutput{
		Summary: summary,
		Source:  "caller_token_quota_usage",
		Status:  "complete",
		Caller:  &info,
	}, nil
}

func callerTokenQuotaInfoFromQuota(quota CallerTokenQuota) callerTokenQuotaInfo {
	usages := make([]tokenQuotaUsageInfo, 0, len(quota.Usages))
	for _, usage := range quota.Usages {
		usages = append(usages, tokenQuotaUsageInfo{
			PolicyID:        usage.PolicyID,
			PolicyName:      usage.PolicyName,
			Period:          usage.Period,
			UsedTokens:      usage.UsedTokens,
			LimitTokens:     usage.LimitTokens,
			RemainingTokens: usage.RemainingTokens,
			StartedAt:       quotaTime(usage.StartedAt),
			ResetsAt:        quotaTime(usage.ResetsAt),
		})
	}
	return callerTokenQuotaInfo{
		ID:      quota.CallerID,
		Name:    quota.CallerName,
		Enabled: quota.Enabled,
		Limited: len(usages) > 0,
		Usages:  usages,
	}
}

func quotaTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func callerTokenQuotaInputResult(err error) (callerTokenQuotaOutput, error) {
	reason, ok := invalidInputReason(err)
	if !ok {
		return callerTokenQuotaOutput{}, err
	}
	return callerTokenQuotaOutput{
		Summary: reason,
		Status:  "invalid_input",
	}, nil
}
