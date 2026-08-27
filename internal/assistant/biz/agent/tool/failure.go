package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const (
	defaultFailureLimit = 10
	maxFailureLimit     = 20
)

type recentFailuresInput struct {
	ResourceType string `json:"resource_type,omitempty" jsonschema_description:"查询全部资源时省略或填 all；限定资源时填 gateway、route 或 service"`
	ResourceID   string `json:"resource_id,omitempty" jsonschema_description:"resource_type 为 gateway、route 或 service 时必填；先用对应的 list 工具取得资源 ID"`
	Outcome      string `json:"outcome,omitempty" jsonschema_description:"失败类型，可填 client_error、server_error 或 no_response，默认 server_error"`
	Hours        int32  `json:"hours,omitempty" jsonschema_description:"查询最近多少小时，默认 24，最大 168"`
	Limit        int32  `json:"limit,omitempty" jsonschema_description:"最多返回多少条记录，默认 10，最大 20"`
}

type failureToolOutput struct {
	Summary      string        `json:"summary"`
	Source       string        `json:"source"`
	Status       string        `json:"status"`
	ResourceType string        `json:"resource_type"`
	ResourceID   string        `json:"resource_id,omitempty"`
	Outcome      string        `json:"outcome"`
	StartTime    string        `json:"start_time"`
	EndTime      string        `json:"end_time"`
	HasMore      bool          `json:"has_more"`
	Items        []failureInfo `json:"items"`
}

type failureInfo struct {
	StartedAt      string  `json:"started_at"`
	Method         string  `json:"method"`
	StatusCode     uint32  `json:"status_code"`
	DurationMillis float64 `json:"duration_millis"`
	GatewayID      string  `json:"gateway_id,omitempty"`
	RouteID        string  `json:"route_id,omitempty"`
	ServiceID      string  `json:"service_id,omitempty"`
}

func newFailureTool(resources FailureReader) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		listRecentFailuresTool,
		"查询近期客户端错误、服务端错误或无响应请求的排障元数据，可限定 gateway、route 或 service 资源。",
		func(ctx context.Context, input recentFailuresInput) (failureToolOutput, error) {
			return listRecentFailures(ctx, resources, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", listRecentFailuresTool, err)
	}
	return definition, nil
}

func listRecentFailures(
	ctx context.Context,
	resources FailureReader,
	input recentFailuresInput,
) (failureToolOutput, error) {
	resourceType, resourceID, err := normalizeResourceScope(input.ResourceType, input.ResourceID)
	if err != nil {
		if reason, ok := invalidInputReason(err); ok {
			return invalidFailureInput(reason), nil
		}
		return failureToolOutput{}, err
	}
	outcomeName, outcome, err := requestOutcome(input.Outcome)
	if err != nil {
		if reason, ok := invalidInputReason(err); ok {
			return invalidFailureInput(reason), nil
		}
		return failureToolOutput{}, err
	}
	hours := observationHours(input.Hours)
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(hours) * time.Hour)
	result, err := resources.ListFailures(ctx, FailureQuery{
		StartTime:    startTime,
		EndTime:      endTime,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      outcome,
		Limit:        failureLimit(input.Limit),
	})
	if err != nil {
		return failureToolOutput{}, err
	}
	items := make([]failureInfo, 0, len(result.Items))
	for _, record := range result.Items {
		items = append(items, failureInfo{
			StartedAt:      record.StartedAt.Format(time.RFC3339),
			Method:         record.Method,
			StatusCode:     record.StatusCode,
			DurationMillis: durationMillis(record.Duration),
			GatewayID:      record.GatewayID,
			RouteID:        record.RouteID,
			ServiceID:      record.ServiceID,
		})
	}
	return failureToolOutput{
		Summary: fmt.Sprintf(
			"最近 %d 小时找到 %d 条%s请求记录",
			hours,
			len(items),
			outcomeLabel(outcomeName),
		),
		Source:       "request_records",
		Status:       resultStatus(result.HasMore),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      outcomeName,
		StartTime:    startTime.Format(time.RFC3339),
		EndTime:      endTime.Format(time.RFC3339),
		HasMore:      result.HasMore,
		Items:        items,
	}, nil
}

func invalidFailureInput(reason string) failureToolOutput {
	// 返回与成功调用相同的结果类型，避免由 Middleware 改写工具语义。
	// Agent 只需要根据 status 和 summary 修正下一次调用。
	return failureToolOutput{
		Summary: reason,
		Status:  "invalid_input",
	}
}

func requestOutcome(value string) (string, FailureOutcome, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "server_error":
		return "server_error", FailureOutcomeServerError, nil
	case "client_error":
		return value, FailureOutcomeClientError, nil
	case "no_response":
		return value, FailureOutcomeNoResponse, nil
	default:
		return "", "", invalidInputf(
			"unsupported outcome %q; use client_error, server_error, or no_response",
			value,
		)
	}
}

func outcomeLabel(outcome string) string {
	switch outcome {
	case "client_error":
		return "客户端错误"
	case "no_response":
		return "无响应"
	default:
		return "服务端错误"
	}
}

func failureLimit(limit int32) int32 {
	if limit <= 0 {
		return defaultFailureLimit
	}
	return min(limit, maxFailureLimit)
}
