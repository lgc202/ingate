package aiusage

import (
	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	aiusagebiz "github.com/lgc202/ingate/internal/analytics/biz/aiusage"
)

func trendResponse(result aiusagebiz.TrendResult) *analyticsv1.GetAIUsageTrendResponse {
	return &analyticsv1.GetAIUsageTrendResponse{
		Summary: metricsResponse(result.Summary),
		Points: lo.Map(result.Points, func(point aiusagebiz.TrendPoint, _ int) *analyticsv1.AIUsageTrendPoint {
			return &analyticsv1.AIUsageTrendPoint{
				StartedAt: timestamppb.New(point.StartedAt),
				Metrics:   metricsResponse(point.Metrics),
			}
		}),
	}
}

func breakdownResponse(items []aiusagebiz.BreakdownItem) *analyticsv1.ListAIUsageBreakdownResponse {
	return &analyticsv1.ListAIUsageBreakdownResponse{
		Items: lo.Map(items, func(item aiusagebiz.BreakdownItem, _ int) *analyticsv1.AIUsageBreakdownItem {
			return &analyticsv1.AIUsageBreakdownItem{
				DimensionValue: item.DimensionValue,
				Metrics:        metricsResponse(item.Metrics),
			}
		}),
	}
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
