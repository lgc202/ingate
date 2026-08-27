package tool

import (
	"context"
	"fmt"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const (
	defaultObservationHours = 24
	maxObservationHours     = 24 * 7
)

type recentTrafficInput struct {
	ResourceType string `json:"resource_type,omitempty" jsonschema_description:"查询全部流量时省略或填 all；限定资源时填 gateway、route 或 service"`
	ResourceID   string `json:"resource_id,omitempty" jsonschema_description:"resource_type 为 gateway、route 或 service 时必填；先用对应的 list 工具取得资源 ID"`
	Hours        int32  `json:"hours,omitempty" jsonschema_description:"查询最近多少小时，默认 24，最大 168"`
}

type trafficToolOutput struct {
	Summary      string             `json:"summary"`
	Source       string             `json:"source"`
	Status       string             `json:"status"`
	ResourceType string             `json:"resource_type"`
	ResourceID   string             `json:"resource_id,omitempty"`
	StartTime    string             `json:"start_time"`
	EndTime      string             `json:"end_time"`
	Metrics      trafficMetricsInfo `json:"metrics"`
}

type trafficMetricsInfo struct {
	RequestCount     uint64  `json:"request_count"`
	NonErrorCount    uint64  `json:"non_error_count"`
	ClientErrorCount uint64  `json:"client_error_count"`
	ServerErrorCount uint64  `json:"server_error_count"`
	NoResponseCount  uint64  `json:"no_response_count"`
	AverageMillis    float64 `json:"average_millis"`
	P50Millis        float64 `json:"p50_millis"`
	P95Millis        float64 `json:"p95_millis"`
	P99Millis        float64 `json:"p99_millis"`
}

func newTrafficTool(resources TrafficReader) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		getRecentTrafficTool,
		"查询最近一段时间的请求量、错误和延迟，可限定 gateway、route 或 service 资源。",
		func(ctx context.Context, input recentTrafficInput) (trafficToolOutput, error) {
			return getRecentTraffic(ctx, resources, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", getRecentTrafficTool, err)
	}
	return definition, nil
}

func getRecentTraffic(
	ctx context.Context,
	resources TrafficReader,
	input recentTrafficInput,
) (trafficToolOutput, error) {
	resourceType, resourceID, err := normalizeResourceScope(input.ResourceType, input.ResourceID)
	if err != nil {
		if reason, ok := invalidInputReason(err); ok {
			return invalidTrafficInput(reason), nil
		}
		return trafficToolOutput{}, err
	}
	hours := observationHours(input.Hours)
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(hours) * time.Hour)
	result, err := resources.GetTraffic(ctx, TrafficQuery{
		StartTime:    startTime,
		EndTime:      endTime,
		ResourceType: resourceType,
		ResourceID:   resourceID,
	})
	if err != nil {
		return trafficToolOutput{}, err
	}
	metrics := trafficMetricsInfo{
		RequestCount:     result.RequestCount,
		NonErrorCount:    result.NonErrorCount,
		ClientErrorCount: result.ClientErrorCount,
		ServerErrorCount: result.ServerErrorCount,
		NoResponseCount:  result.NoResponseCount,
		AverageMillis:    durationMillis(result.AverageDuration),
		P50Millis:        durationMillis(result.P50Duration),
		P95Millis:        durationMillis(result.P95Duration),
		P99Millis:        durationMillis(result.P99Duration),
	}
	return trafficToolOutput{
		Summary: fmt.Sprintf(
			"最近 %d 小时共 %d 次请求，其中客户端错误 %d 次、服务端错误 %d 次、无响应 %d 次",
			hours,
			metrics.RequestCount,
			metrics.ClientErrorCount,
			metrics.ServerErrorCount,
			metrics.NoResponseCount,
		),
		Source:       "traffic_analysis",
		Status:       "complete",
		ResourceType: resourceType,
		ResourceID:   resourceID,
		StartTime:    startTime.Format(time.RFC3339),
		EndTime:      endTime.Format(time.RFC3339),
		Metrics:      metrics,
	}, nil
}

func invalidTrafficInput(reason string) trafficToolOutput {
	// 参数约束是工具协议的一部分。把可修正原因作为正常工具结果
	// 返回 Eino，由 Agent 判断是否补全资源 ID 后再调用；系统错误仍然向上返回。
	return trafficToolOutput{
		Summary: reason,
		Status:  "invalid_input",
	}
}

func durationMillis(value time.Duration) float64 {
	return float64(value) / float64(time.Millisecond)
}

func observationHours(hours int32) int32 {
	if hours <= 0 {
		return defaultObservationHours
	}
	return min(hours, maxObservationHours)
}
