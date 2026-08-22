package aiusage

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	aiusagebiz "github.com/lgc202/ingate/internal/analytics/biz/aiusage"
)

func trendResponse(result aiusagebiz.TrendResult) *analyticsv1.GetAIUsageTrendResponse {
	response := &analyticsv1.GetAIUsageTrendResponse{
		Summary: metricsResponse(result.Summary),
		Points:  make([]*analyticsv1.AIUsageTrendPoint, 0, len(result.Points)),
	}
	for _, point := range result.Points {
		response.Points = append(response.Points, &analyticsv1.AIUsageTrendPoint{
			StartedAt: timestamppb.New(point.StartedAt),
			Metrics:   metricsResponse(point.Metrics),
		})
	}
	return response
}

func breakdownResponse(items []aiusagebiz.BreakdownItem) *analyticsv1.ListAIUsageBreakdownResponse {
	response := &analyticsv1.ListAIUsageBreakdownResponse{
		Items: make([]*analyticsv1.AIUsageBreakdownItem, 0, len(items)),
	}
	for _, item := range items {
		response.Items = append(response.Items, &analyticsv1.AIUsageBreakdownItem{
			DimensionValue: item.DimensionValue,
			Metrics:        metricsResponse(item.Metrics),
		})
	}
	return response
}

func metricsResponse(metrics aiusagebiz.Metrics) *analyticsv1.AIUsageMetrics {
	return &analyticsv1.AIUsageMetrics{
		CallCount:              metrics.CallCount,
		NormalResponseCount:    metrics.NormalResponseCount,
		TokenReportedCallCount: metrics.TokenReportedCallCount,
		InputTokens:            metrics.InputTokens,
		OutputTokens:           metrics.OutputTokens,
		TotalTokens:            metrics.TotalTokens,
	}
}
