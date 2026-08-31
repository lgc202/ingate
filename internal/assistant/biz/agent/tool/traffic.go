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
	defaultTrafficLimit = 10
	maxTrafficLimit     = 50
)

type trafficAnalysisInput struct {
	ScopeType string `json:"scope_type,omitempty" jsonschema_description:"查询全部流量时省略或填 all；限定范围时填 gateway、route 或 service"`
	ScopeID   string `json:"scope_id,omitempty" jsonschema_description:"scope_type 为 gateway、route 或 service 时必填；先用对应的 list 工具取得资源 ID"`
	GroupBy   string `json:"group_by,omitempty" jsonschema_description:"排名维度，可填 gateway、route 或 service，默认 route"`
	OrderBy   string `json:"order_by,omitempty" jsonschema_description:"排序依据，可填 request_count、server_error_rate 或 p95_duration，默认 request_count"`
	Limit     int32  `json:"limit,omitempty" jsonschema_description:"最多返回多少个资源，默认 10，最大 50"`
	Hours     int32  `json:"hours,omitempty" jsonschema_description:"查询最近多少小时，默认 24，最大 168；与 start_time、end_time 二选一"`
	StartTime string `json:"start_time,omitempty" jsonschema_description:"自定义范围起点，RFC3339 格式；必须和 end_time 一起提供"`
	EndTime   string `json:"end_time,omitempty" jsonschema_description:"自定义范围终点，RFC3339 格式；必须和 start_time 一起提供"`
}

type trafficAnalysisOutput struct {
	Summary   string                `json:"summary"`
	Source    string                `json:"source,omitempty"`
	Status    string                `json:"status"`
	ScopeType string                `json:"scope_type,omitempty"`
	ScopeID   string                `json:"scope_id,omitempty"`
	GroupBy   TrafficDimension      `json:"group_by,omitempty"`
	OrderBy   TrafficOrder          `json:"order_by,omitempty"`
	StartTime string                `json:"start_time,omitempty"`
	EndTime   string                `json:"end_time,omitempty"`
	Metrics   trafficMetricsInfo    `json:"metrics"`
	Ranking   []trafficResourceInfo `json:"ranking,omitempty"`
}

type trafficResourceInfo struct {
	ResourceID   string             `json:"resource_id"`
	ResourceName string             `json:"resource_name"`
	Metrics      trafficMetricsInfo `json:"metrics"`
}

// trafficMetricsInfo 中的错误率采用 0 到 1 之间的小数，
// 模型在面向用户展示时可以按百分比格式化。
type trafficMetricsInfo struct {
	RequestCount     uint64  `json:"request_count"`
	NonErrorCount    uint64  `json:"non_error_count"`
	ClientErrorCount uint64  `json:"client_error_count"`
	ClientErrorRate  float64 `json:"client_error_rate"`
	ServerErrorCount uint64  `json:"server_error_count"`
	ServerErrorRate  float64 `json:"server_error_rate"`
	NoResponseCount  uint64  `json:"no_response_count"`
	AverageMillis    float64 `json:"average_millis"`
	P50Millis        float64 `json:"p50_millis"`
	P95Millis        float64 `json:"p95_millis"`
	P99Millis        float64 `json:"p99_millis"`
}

// TrafficReader 是流量分析工具实际使用的查询边界。
type TrafficReader interface {
	AnalyzeTraffic(context.Context, TrafficQuery) (TrafficAnalysis, error)
}

func newTrafficTool(resources TrafficReader) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		analyzeTrafficTool,
		"分析指定时间和资源范围内的流量，可按网关、路由或服务分组，并按请求量、服务端错误率或 P95 耗时排序。一条调用即可得到资源排名，不要逐个资源重复查询。",
		func(ctx context.Context, input trafficAnalysisInput) (trafficAnalysisOutput, error) {
			return analyzeTraffic(ctx, resources, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", analyzeTrafficTool, err)
	}
	return definition, nil
}

func analyzeTraffic(
	ctx context.Context,
	resources TrafficReader,
	input trafficAnalysisInput,
) (trafficAnalysisOutput, error) {
	scopeType, scopeID, err := normalizeResourceScope(input.ScopeType, input.ScopeID)
	if err != nil {
		return trafficInputResult(err)
	}
	groupBy, err := normalizeTrafficDimension(input.GroupBy)
	if err != nil {
		return trafficInputResult(err)
	}
	orderBy, err := normalizeTrafficOrder(input.OrderBy)
	if err != nil {
		return trafficInputResult(err)
	}
	limit, err := trafficLimit(input.Limit)
	if err != nil {
		return trafficInputResult(err)
	}
	startTime, endTime, err := observationTimeRange(input.Hours, input.StartTime, input.EndTime)
	if err != nil {
		return trafficInputResult(err)
	}

	result, err := resources.AnalyzeTraffic(ctx, TrafficQuery{
		StartTime: startTime,
		EndTime:   endTime,
		ScopeType: scopeType,
		ScopeID:   scopeID,
		GroupBy:   groupBy,
		OrderBy:   orderBy,
		Limit:     limit,
	})
	if err != nil {
		return trafficAnalysisOutput{}, err
	}

	ranking := make([]trafficResourceInfo, 0, len(result.Items))
	for _, item := range result.Items {
		ranking = append(ranking, trafficResourceInfo{
			ResourceID:   item.ID,
			ResourceName: item.Name,
			Metrics:      trafficMetrics(item.Metrics),
		})
	}
	return trafficAnalysisOutput{
		Summary: fmt.Sprintf(
			"流量分析完成，共统计 %d 次请求，返回 %d 个 %s 排名结果",
			result.Summary.RequestCount,
			len(ranking),
			result.GroupBy,
		),
		Source:    "traffic_analysis",
		Status:    "complete",
		ScopeType: scopeType,
		ScopeID:   scopeID,
		GroupBy:   result.GroupBy,
		OrderBy:   result.OrderBy,
		StartTime: startTime.Format(time.RFC3339Nano),
		EndTime:   endTime.Format(time.RFC3339Nano),
		Metrics:   trafficMetrics(result.Summary),
		Ranking:   ranking,
	}, nil
}

func trafficInputResult(err error) (trafficAnalysisOutput, error) {
	reason, ok := invalidInputReason(err)
	if !ok {
		return trafficAnalysisOutput{}, err
	}
	// 参数约束是工具协议的一部分。把可修正原因返回 Eino，Agent 可以在
	// 同一轮补全参数后重试；网络和存储错误仍沿调用链返回到日志边界。
	return trafficAnalysisOutput{
		Summary: reason,
		Status:  "invalid_input",
	}, nil
}

func normalizeTrafficDimension(value string) (TrafficDimension, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(TrafficDimensionRoute):
		return TrafficDimensionRoute, nil
	case string(TrafficDimensionGateway):
		return TrafficDimensionGateway, nil
	case string(TrafficDimensionService):
		return TrafficDimensionService, nil
	default:
		return "", invalidInputf("unsupported group_by %q; use gateway, route, or service", value)
	}
}

func normalizeTrafficOrder(value string) (TrafficOrder, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(TrafficOrderRequestCount):
		return TrafficOrderRequestCount, nil
	case string(TrafficOrderServerErrorRate):
		return TrafficOrderServerErrorRate, nil
	case string(TrafficOrderP95Duration):
		return TrafficOrderP95Duration, nil
	default:
		return "", invalidInputf(
			"unsupported order_by %q; use request_count, server_error_rate, or p95_duration",
			value,
		)
	}
}

func trafficLimit(value int32) (uint32, error) {
	if value < 0 || value > maxTrafficLimit {
		return 0, invalidInputf("limit must be omitted or between 1 and %d", maxTrafficLimit)
	}
	if value == 0 {
		return defaultTrafficLimit, nil
	}
	return uint32(value), nil
}

func trafficMetrics(metrics TrafficMetrics) trafficMetricsInfo {
	return trafficMetricsInfo{
		RequestCount:     metrics.RequestCount,
		NonErrorCount:    metrics.NonErrorCount,
		ClientErrorCount: metrics.ClientErrorCount,
		ClientErrorRate:  requestRate(metrics.ClientErrorCount, metrics.RequestCount),
		ServerErrorCount: metrics.ServerErrorCount,
		ServerErrorRate:  requestRate(metrics.ServerErrorCount, metrics.RequestCount),
		NoResponseCount:  metrics.NoResponseCount,
		AverageMillis:    durationMillis(metrics.AverageDuration),
		P50Millis:        durationMillis(metrics.P50Duration),
		P95Millis:        durationMillis(metrics.P95Duration),
		P99Millis:        durationMillis(metrics.P99Duration),
	}
}

func requestRate(count, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total)
}

func durationMillis(value time.Duration) float64 {
	return float64(value) / float64(time.Millisecond)
}
