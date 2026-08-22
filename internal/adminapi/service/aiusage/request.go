package aiusage

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	aiusagebiz "github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

const maximumTimeRange = 90 * 24 * time.Hour

func analysisQuery(request *adminv1.GetAIUsageAnalysisRequest) (aiusagebiz.Query, error) {
	startTime, endTime, err := analysisTimeRange(request.GetStartTime(), request.GetEndTime())
	if err != nil {
		return aiusagebiz.Query{}, err
	}
	return aiusagebiz.Query{
		Filter: aiusagebiz.Filter{
			StartTime:   startTime,
			EndTime:     endTime,
			GatewayID:   request.GetGatewayId(),
			CallerID:    request.GetCallerId(),
			RouteID:     request.GetRouteId(),
			ClientModel: request.GetClientModel(),
			ServiceID:   request.GetServiceId(),
			ActualModel: request.GetActualModel(),
		},
		Dimension: breakdownDimension(request.GetBreakdownDimension()),
		Order:     breakdownOrder(request.GetBreakdownOrder()),
		Limit:     int(request.GetBreakdownLimit()),
	}, nil
}

func analysisTimeRange(start, end *timestamppb.Timestamp) (time.Time, time.Time, error) {
	if start == nil || start.CheckValid() != nil {
		return time.Time{}, time.Time{}, adminservice.BadRequest("请选择查询开始时间")
	}
	if end == nil || end.CheckValid() != nil {
		return time.Time{}, time.Time{}, adminservice.BadRequest("请选择查询结束时间")
	}
	startTime := alignStartTime(start.AsTime())
	endTime := alignEndTime(end.AsTime())
	if !startTime.Before(endTime) {
		return time.Time{}, time.Time{}, adminservice.BadRequest("查询开始时间必须早于结束时间")
	}
	if endTime.Sub(startTime) > maximumTimeRange {
		return time.Time{}, time.Time{}, adminservice.BadRequest("单次最多查询 90 天 AI 用量")
	}
	return startTime, endTime, nil
}

func alignStartTime(value time.Time) time.Time {
	return value.Truncate(time.Minute)
}

func alignEndTime(value time.Time) time.Time {
	aligned := value.Truncate(time.Minute)
	if aligned.Equal(value) {
		return aligned
	}
	return aligned.Add(time.Minute)
}

func breakdownDimension(value adminv1.AIUsageBreakdownDimension) aiusagebiz.Dimension {
	switch value {
	case adminv1.AIUsageBreakdownDimension_AI_USAGE_BREAKDOWN_DIMENSION_ROUTE:
		return aiusagebiz.DimensionRoute
	case adminv1.AIUsageBreakdownDimension_AI_USAGE_BREAKDOWN_DIMENSION_CLIENT_MODEL:
		return aiusagebiz.DimensionClientModel
	case adminv1.AIUsageBreakdownDimension_AI_USAGE_BREAKDOWN_DIMENSION_SERVICE:
		return aiusagebiz.DimensionService
	case adminv1.AIUsageBreakdownDimension_AI_USAGE_BREAKDOWN_DIMENSION_ACTUAL_MODEL:
		return aiusagebiz.DimensionActualModel
	default:
		return aiusagebiz.DimensionCaller
	}
}

func breakdownOrder(value adminv1.AIUsageBreakdownOrder) aiusagebiz.BreakdownOrder {
	if value == adminv1.AIUsageBreakdownOrder_AI_USAGE_BREAKDOWN_ORDER_TOTAL_TOKENS {
		return aiusagebiz.BreakdownOrderTotalTokens
	}
	return aiusagebiz.BreakdownOrderCallCount
}
