package analytics

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	aiusagebiz "github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
)

func analyticsAIUsageFilter(filter aiusagebiz.Filter) *analyticsv1.AIUsageFilter {
	return &analyticsv1.AIUsageFilter{
		StartTime:     timestamppb.New(filter.StartTime),
		EndTime:       timestamppb.New(filter.EndTime),
		GatewayId:     filter.GatewayID,
		CallerId:      filter.CallerID,
		RouteId:       filter.RouteID,
		ClientModel:   filter.ClientModel,
		UpstreamId:    filter.ServiceID,
		UpstreamModel: filter.ActualModel,
	}
}

func aiUsageMetrics(metrics *analyticsv1.AIUsageMetrics) (aiusagebiz.Metrics, error) {
	if metrics == nil {
		return aiusagebiz.Metrics{}, errors.New("analytics returned empty AI usage metrics")
	}
	if metrics.GetNormalResponseCount() > metrics.GetCallCount() ||
		metrics.GetTokenReportedCallCount() > metrics.GetCallCount() {
		return aiusagebiz.Metrics{}, errors.New("analytics AI usage counts are inconsistent")
	}
	return aiusagebiz.Metrics{
		CallCount:              metrics.GetCallCount(),
		NormalResponseCount:    metrics.GetNormalResponseCount(),
		TokenReportedCallCount: metrics.GetTokenReportedCallCount(),
		InputTokens:            metrics.GetInputTokens(),
		OutputTokens:           metrics.GetOutputTokens(),
		TotalTokens:            metrics.GetTotalTokens(),
	}, nil
}

func aiUsageTrend(
	points []*analyticsv1.AIUsageTrendPoint,
	filter aiusagebiz.Filter,
) ([]aiusagebiz.TrendPoint, error) {
	trend := make([]aiusagebiz.TrendPoint, len(points))
	var previousStart time.Time
	for i, point := range points {
		if point == nil || point.GetStartedAt() == nil || point.GetStartedAt().CheckValid() != nil {
			return nil, errors.New("analytics returned invalid AI usage trend point")
		}
		startedAt := point.GetStartedAt().AsTime()
		if startedAt.Before(filter.StartTime) || !startedAt.Before(filter.EndTime) {
			return nil, errors.New("analytics returned an AI usage trend point outside the query range")
		}
		if i > 0 && !startedAt.After(previousStart) {
			return nil, errors.New("analytics returned unordered AI usage trend points")
		}
		metrics, err := aiUsageMetrics(point.GetMetrics())
		if err != nil {
			return nil, fmt.Errorf("convert AI usage trend point: %w", err)
		}
		trend[i] = aiusagebiz.TrendPoint{
			StartedAt: startedAt,
			Metrics:   metrics,
		}
		previousStart = startedAt
	}
	return trend, nil
}

func aiUsageBreakdown(items []*analyticsv1.AIUsageBreakdownItem) ([]aiusagebiz.BreakdownItem, error) {
	breakdown := make([]aiusagebiz.BreakdownItem, len(items))
	seen := make(map[string]bool, len(items))
	for i, item := range items {
		if item == nil || item.GetDimensionValue() == "" {
			return nil, errors.New("analytics returned invalid AI usage breakdown item")
		}
		dimensionValue := item.GetDimensionValue()
		if seen[dimensionValue] {
			return nil, errors.New("analytics returned duplicate AI usage breakdown items")
		}
		seen[dimensionValue] = true
		metrics, err := aiUsageMetrics(item.GetMetrics())
		if err != nil {
			return nil, fmt.Errorf("convert AI usage breakdown item: %w", err)
		}
		breakdown[i] = aiusagebiz.BreakdownItem{
			DimensionValue: dimensionValue,
			Metrics:        metrics,
		}
	}
	return breakdown, nil
}

func analyticsAIUsageTimeBucket(value aiusagebiz.TimeBucket) analyticsv1.TimeBucket {
	switch value {
	case aiusagebiz.TimeBucketMinute:
		return analyticsv1.TimeBucket_TIME_BUCKET_MINUTE
	case aiusagebiz.TimeBucketFiveMinutes:
		return analyticsv1.TimeBucket_TIME_BUCKET_FIVE_MINUTES
	case aiusagebiz.TimeBucketHour:
		return analyticsv1.TimeBucket_TIME_BUCKET_HOUR
	default:
		return analyticsv1.TimeBucket_TIME_BUCKET_DAY
	}
}

func analyticsAIUsageDimension(value aiusagebiz.Dimension) analyticsv1.AIUsageDimension {
	switch value {
	case aiusagebiz.DimensionCaller:
		return analyticsv1.AIUsageDimension_AI_USAGE_DIMENSION_CALLER
	case aiusagebiz.DimensionRoute:
		return analyticsv1.AIUsageDimension_AI_USAGE_DIMENSION_ROUTE
	case aiusagebiz.DimensionClientModel:
		return analyticsv1.AIUsageDimension_AI_USAGE_DIMENSION_CLIENT_MODEL
	case aiusagebiz.DimensionService:
		return analyticsv1.AIUsageDimension_AI_USAGE_DIMENSION_UPSTREAM
	default:
		return analyticsv1.AIUsageDimension_AI_USAGE_DIMENSION_UPSTREAM_MODEL
	}
}

func analyticsAIUsageBreakdownOrder(value aiusagebiz.BreakdownOrder) analyticsv1.AIUsageBreakdownOrder {
	switch value {
	case aiusagebiz.BreakdownOrderTotalTokens:
		return analyticsv1.AIUsageBreakdownOrder_AI_USAGE_BREAKDOWN_ORDER_TOTAL_TOKENS
	default:
		return analyticsv1.AIUsageBreakdownOrder_AI_USAGE_BREAKDOWN_ORDER_CALL_COUNT
	}
}
