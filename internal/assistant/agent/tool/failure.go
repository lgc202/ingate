package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
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

func newFailureTool(resources ResourceReader) (einotool.BaseTool, error) {
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
	resources ResourceReader,
	input recentFailuresInput,
) (failureToolOutput, error) {
	resourceType, resourceID, err := normalizeResourceScope(input.ResourceType, input.ResourceID)
	if err != nil {
		return failureToolOutput{}, err
	}
	outcomeName, outcome, err := requestOutcome(input.Outcome)
	if err != nil {
		return failureToolOutput{}, err
	}
	hours := observationHours(input.Hours)
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(hours) * time.Hour)
	request := &adminv1.ListRequestRecordsRequest{
		StartTime: timestamppb.New(startTime),
		EndTime:   timestamppb.New(endTime),
		Outcome:   outcome,
		PageSize:  failureLimit(input.Limit),
	}
	applyRequestResourceScope(request, resourceType, resourceID)
	result, err := resources.ListRequestRecords(ctx, request)
	if err != nil {
		return failureToolOutput{}, err
	}
	items := make([]failureInfo, 0, len(result.GetRecords()))
	for _, record := range result.GetRecords() {
		startedAt := ""
		if record.GetStartedAt() != nil {
			startedAt = record.GetStartedAt().AsTime().Format(time.RFC3339)
		}
		items = append(items, failureInfo{
			StartedAt:      startedAt,
			Method:         record.GetMethod(),
			StatusCode:     record.GetStatusCode(),
			DurationMillis: durationMillis(record.GetDuration()),
			GatewayID:      record.GetGatewayId(),
			RouteID:        record.GetRouteId(),
			ServiceID:      record.GetServiceId(),
		})
	}
	hasMore := result.GetNextPageToken() != ""
	return failureToolOutput{
		Summary: fmt.Sprintf(
			"最近 %d 小时找到 %d 条%s请求记录",
			hours,
			len(items),
			outcomeLabel(outcomeName),
		),
		Source:       "request_records",
		Status:       resultStatus(hasMore),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      outcomeName,
		StartTime:    startTime.Format(time.RFC3339),
		EndTime:      endTime.Format(time.RFC3339),
		HasMore:      hasMore,
		Items:        items,
	}, nil
}

func requestOutcome(value string) (string, adminv1.RequestOutcome, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "server_error":
		return "server_error", adminv1.RequestOutcome_REQUEST_OUTCOME_SERVER_ERROR, nil
	case "client_error":
		return value, adminv1.RequestOutcome_REQUEST_OUTCOME_CLIENT_ERROR, nil
	case "no_response":
		return value, adminv1.RequestOutcome_REQUEST_OUTCOME_NO_RESPONSE, nil
	default:
		return "", adminv1.RequestOutcome_REQUEST_OUTCOME_UNSPECIFIED, invalidInputf(
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

func applyRequestResourceScope(
	request *adminv1.ListRequestRecordsRequest,
	resourceType string,
	resourceID string,
) {
	switch resourceType {
	case "gateway":
		request.GatewayId = resourceID
	case "route":
		request.RouteId = resourceID
	case "service":
		request.ServiceId = resourceID
	}
}
