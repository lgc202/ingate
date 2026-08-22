package aiusage

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	aiusagebiz "github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
)

func analysisResponse(analysis aiusagebiz.Analysis) *adminv1.GetAIUsageAnalysisResponse {
	response := &adminv1.GetAIUsageAnalysisResponse{
		Summary:            metricsResponse(analysis.Summary),
		BreakdownDimension: dimensionResponse(analysis.Dimension),
		BreakdownOrder:     orderResponse(analysis.Order),
		Trend:              make([]*adminv1.AIUsageAnalysisPoint, 0, len(analysis.Trend)),
		Breakdown:          make([]*adminv1.AIUsageBreakdownItem, 0, len(analysis.Breakdown)),
	}
	for _, point := range analysis.Trend {
		response.Trend = append(response.Trend, &adminv1.AIUsageAnalysisPoint{
			StartedAt: timestamppb.New(point.StartedAt),
			Metrics:   metricsResponse(point.Metrics),
		})
	}
	for _, item := range analysis.Breakdown {
		response.Breakdown = append(response.Breakdown, &adminv1.AIUsageBreakdownItem{
			DimensionValue: item.DimensionValue,
			Metrics:        metricsResponse(item.Metrics),
		})
	}
	return response
}

func metricsResponse(metrics aiusagebiz.Metrics) *adminv1.AIUsageMetrics {
	return &adminv1.AIUsageMetrics{
		CallCount:              metrics.CallCount,
		NormalResponseCount:    metrics.NormalResponseCount,
		TokenReportedCallCount: metrics.TokenReportedCallCount,
		InputTokens:            metrics.InputTokens,
		OutputTokens:           metrics.OutputTokens,
		TotalTokens:            metrics.TotalTokens,
	}
}

func dimensionResponse(value aiusagebiz.Dimension) adminv1.AIUsageBreakdownDimension {
	switch value {
	case aiusagebiz.DimensionRoute:
		return adminv1.AIUsageBreakdownDimension_AI_USAGE_BREAKDOWN_DIMENSION_ROUTE
	case aiusagebiz.DimensionClientModel:
		return adminv1.AIUsageBreakdownDimension_AI_USAGE_BREAKDOWN_DIMENSION_CLIENT_MODEL
	case aiusagebiz.DimensionService:
		return adminv1.AIUsageBreakdownDimension_AI_USAGE_BREAKDOWN_DIMENSION_SERVICE
	case aiusagebiz.DimensionActualModel:
		return adminv1.AIUsageBreakdownDimension_AI_USAGE_BREAKDOWN_DIMENSION_ACTUAL_MODEL
	default:
		return adminv1.AIUsageBreakdownDimension_AI_USAGE_BREAKDOWN_DIMENSION_CALLER
	}
}

func orderResponse(value aiusagebiz.BreakdownOrder) adminv1.AIUsageBreakdownOrder {
	if value == aiusagebiz.BreakdownOrderTotalTokens {
		return adminv1.AIUsageBreakdownOrder_AI_USAGE_BREAKDOWN_ORDER_TOTAL_TOKENS
	}
	return adminv1.AIUsageBreakdownOrder_AI_USAGE_BREAKDOWN_ORDER_CALL_COUNT
}
