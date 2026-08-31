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
	ScopeType string `json:"scope_type,omitempty" jsonschema_description:"查询全部资源时省略或填 all；限定范围时填 gateway、route 或 service"`
	ScopeID   string `json:"scope_id,omitempty" jsonschema_description:"scope_type 为 gateway、route 或 service 时必填；可直接使用 analyze_traffic 排名返回的 resource_id"`
	Outcome   string `json:"outcome,omitempty" jsonschema_description:"失败类型，可填 client_error、server_error 或 no_response，默认 server_error"`
	Limit     int32  `json:"limit,omitempty" jsonschema_description:"最多返回多少条失败样本，默认 10，最大 20"`
	Hours     int32  `json:"hours,omitempty" jsonschema_description:"查询最近多少小时，默认 24，最大 168；与 start_time、end_time 二选一"`
	StartTime string `json:"start_time,omitempty" jsonschema_description:"自定义范围起点，RFC3339 格式；必须和 end_time 一起提供"`
	EndTime   string `json:"end_time,omitempty" jsonschema_description:"自定义范围终点，RFC3339 格式；必须和 start_time 一起提供"`
}

type failureToolOutput struct {
	Summary   string        `json:"summary"`
	Source    string        `json:"source"`
	Status    string        `json:"status"`
	ScopeType string        `json:"scope_type"`
	ScopeID   string        `json:"scope_id,omitempty"`
	ScopeName string        `json:"scope_name,omitempty"`
	Outcome   string        `json:"outcome"`
	StartTime string        `json:"start_time"`
	EndTime   string        `json:"end_time"`
	HasMore   bool          `json:"has_more"`
	Items     []failureInfo `json:"items"`
}

type failureInfo struct {
	RecordID       string  `json:"record_id"`
	StartedAt      string  `json:"started_at"`
	Method         string  `json:"method"`
	Host           string  `json:"host"`
	Path           string  `json:"path"`
	StatusCode     uint32  `json:"status_code"`
	DurationMillis float64 `json:"duration_millis"`
	GatewayID      string  `json:"gateway_id,omitempty"`
	RouteID        string  `json:"route_id,omitempty"`
	ServiceID      string  `json:"service_id,omitempty"`
}

// FailureReader 是失败请求工具实际使用的查询边界。
type FailureReader interface {
	ListFailures(context.Context, FailureQuery) (FailurePage, error)
}

func newFailureTool(resources FailureReader) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		listRecentFailuresTool,
		"查询近期客户端错误、服务端错误或无响应请求样本，可限定 analyze_traffic 返回的网关、路由或服务范围，并返回请求方法、域名、路径、状态码和耗时。",
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
	scopeType, scopeID, err := normalizeResourceScope(input.ScopeType, input.ScopeID)
	if err != nil {
		return failureInputResult(err)
	}
	outcomeName, outcome, err := requestOutcome(input.Outcome)
	if err != nil {
		return failureInputResult(err)
	}
	limit, err := failureLimit(input.Limit)
	if err != nil {
		return failureInputResult(err)
	}
	startTime, endTime, err := observationTimeRange(input.Hours, input.StartTime, input.EndTime)
	if err != nil {
		return failureInputResult(err)
	}
	result, err := resources.ListFailures(ctx, FailureQuery{
		StartTime: startTime,
		EndTime:   endTime,
		ScopeType: scopeType,
		ScopeID:   scopeID,
		Outcome:   outcome,
		Limit:     limit,
	})
	if err != nil {
		return failureToolOutput{}, err
	}
	items := make([]failureInfo, 0, len(result.Items))
	for _, record := range result.Items {
		items = append(items, failureInfo{
			RecordID:       record.RecordID,
			StartedAt:      record.StartedAt.Format(time.RFC3339Nano),
			Method:         record.Method,
			Host:           record.Host,
			Path:           record.Path,
			StatusCode:     record.StatusCode,
			DurationMillis: durationMillis(record.Duration),
			GatewayID:      record.GatewayID,
			RouteID:        record.RouteID,
			ServiceID:      record.ServiceID,
		})
	}
	return failureToolOutput{
		Summary: fmt.Sprintf(
			"指定时间范围内找到 %d 条%s请求记录",
			len(items),
			outcomeLabel(outcomeName),
		),
		Source:    "request_records",
		Status:    resultStatus(result.HasMore),
		ScopeType: scopeType,
		ScopeID:   scopeID,
		ScopeName: result.ScopeName,
		Outcome:   outcomeName,
		StartTime: startTime.Format(time.RFC3339Nano),
		EndTime:   endTime.Format(time.RFC3339Nano),
		HasMore:   result.HasMore,
		Items:     items,
	}, nil
}

func failureInputResult(err error) (failureToolOutput, error) {
	reason, ok := invalidInputReason(err)
	if !ok {
		return failureToolOutput{}, err
	}
	// 返回与成功调用相同的结果类型，避免由 Middleware 改写工具语义。
	// Agent 只需要根据 status 和 summary 修正下一次调用。
	return failureToolOutput{
		Summary: reason,
		Status:  "invalid_input",
	}, nil
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

func failureLimit(value int32) (int32, error) {
	if value < 0 || value > maxFailureLimit {
		return 0, invalidInputf("limit must be omitted or between 1 and %d", maxFailureLimit)
	}
	if value == 0 {
		return defaultFailureLimit, nil
	}
	return value, nil
}
