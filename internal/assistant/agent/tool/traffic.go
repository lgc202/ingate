package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

const (
	defaultObservationHours = 24
	maxObservationHours     = 24 * 7
)

type recentTrafficInput struct {
	ResourceType string `json:"resource_type,omitempty"`
	ResourceID   string `json:"resource_id,omitempty"`
	Hours        int32  `json:"hours,omitempty"`
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

func newTrafficTool(resources ResourceReader) (einotool.BaseTool, error) {
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
	resources ResourceReader,
	input recentTrafficInput,
) (trafficToolOutput, error) {
	resourceType, resourceID, err := normalizeResourceScope(input.ResourceType, input.ResourceID)
	if err != nil {
		return trafficToolOutput{}, err
	}
	hours := observationHours(input.Hours)
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(hours) * time.Hour)
	request := &adminv1.GetTrafficAnalysisRequest{
		StartTime: timestamppb.New(startTime),
		EndTime:   timestamppb.New(endTime),
	}
	applyResourceScope(request, resourceType, resourceID)
	result, err := resources.GetTrafficAnalysis(ctx, request)
	if err != nil {
		return trafficToolOutput{}, err
	}
	metrics := trafficMetrics(result.GetSummary())
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

func trafficMetrics(metrics *adminv1.TrafficMetrics) trafficMetricsInfo {
	return trafficMetricsInfo{
		RequestCount:     metrics.GetRequestCount(),
		NonErrorCount:    metrics.GetNonErrorCount(),
		ClientErrorCount: metrics.GetClientErrorCount(),
		ServerErrorCount: metrics.GetServerErrorCount(),
		NoResponseCount:  metrics.GetNoResponseCount(),
		AverageMillis:    durationMillis(metrics.GetAverageDuration()),
		P50Millis:        durationMillis(metrics.GetP50Duration()),
		P95Millis:        durationMillis(metrics.GetP95Duration()),
		P99Millis:        durationMillis(metrics.GetP99Duration()),
	}
}

func durationMillis(value *durationpb.Duration) float64 {
	if value == nil {
		return 0
	}
	return float64(value.AsDuration()) / float64(time.Millisecond)
}

func observationHours(hours int32) int32 {
	if hours <= 0 {
		return defaultObservationHours
	}
	return min(hours, maxObservationHours)
}

func normalizeResourceScope(resourceType, resourceID string) (string, string, error) {
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	resourceID = strings.TrimSpace(resourceID)
	if resourceType == "" {
		resourceType = "all"
	}
	if resourceType == "all" {
		if resourceID != "" {
			return "", "", fmt.Errorf("resource_id requires gateway, route, or service resource_type")
		}
		return resourceType, resourceID, nil
	}
	if resourceType != "gateway" && resourceType != "route" && resourceType != "service" {
		return "", "", fmt.Errorf("unsupported resource_type %q", resourceType)
	}
	if resourceID == "" {
		return "", "", fmt.Errorf("resource_id is required for %s resource_type", resourceType)
	}
	return resourceType, resourceID, nil
}

func applyResourceScope(request *adminv1.GetTrafficAnalysisRequest, resourceType, resourceID string) {
	switch resourceType {
	case "gateway":
		request.GatewayId = resourceID
	case "route":
		request.RouteId = resourceID
	case "service":
		request.ServiceId = resourceID
	}
}
