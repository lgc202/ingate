package analytics

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
)

type trafficMetricValues struct {
	requestCount    uint64
	clientErrors    uint64
	serverErrors    uint64
	noResponses     uint64
	averageDuration *durationpb.Duration
	p50Duration     *durationpb.Duration
	p95Duration     *durationpb.Duration
	p99Duration     *durationpb.Duration
}

func trafficSummary(summary *analyticsv1.TrafficSummary) (trafficbiz.Metrics, error) {
	if summary == nil {
		return trafficbiz.Metrics{}, errors.New("analytics returned an empty traffic summary")
	}
	return trafficMetrics(trafficMetricValues{
		requestCount:    summary.GetRequestCount(),
		clientErrors:    summary.GetClientErrorCount(),
		serverErrors:    summary.GetServerErrorCount(),
		noResponses:     summary.GetNoResponseCount(),
		averageDuration: summary.GetAverageDuration(),
		p50Duration:     summary.GetP50Duration(),
		p95Duration:     summary.GetP95Duration(),
		p99Duration:     summary.GetP99Duration(),
	})
}

func trafficTrend(points []*analyticsv1.TrafficTrendPoint) ([]trafficbiz.TrendPoint, error) {
	trend := make([]trafficbiz.TrendPoint, 0, len(points))
	for _, point := range points {
		if point == nil || point.GetStartedAt() == nil || point.GetStartedAt().CheckValid() != nil {
			return nil, errors.New("analytics returned an invalid traffic trend point")
		}
		metrics, err := trafficMetrics(trafficMetricValues{
			requestCount:    point.GetRequestCount(),
			clientErrors:    point.GetClientErrorCount(),
			serverErrors:    point.GetServerErrorCount(),
			noResponses:     point.GetNoResponseCount(),
			averageDuration: point.GetAverageDuration(),
			p50Duration:     point.GetP50Duration(),
			p95Duration:     point.GetP95Duration(),
			p99Duration:     point.GetP99Duration(),
		})
		if err != nil {
			return nil, fmt.Errorf("convert traffic trend point: %w", err)
		}
		trend = append(trend, trafficbiz.TrendPoint{StartedAt: point.GetStartedAt().AsTime(), Metrics: metrics})
	}
	return trend, nil
}

func trafficBreakdown(items []*analyticsv1.TrafficBreakdownItem) ([]trafficbiz.BreakdownItem, error) {
	breakdown := make([]trafficbiz.BreakdownItem, 0, len(items))
	for _, item := range items {
		if item == nil || item.GetResourceId() == "" {
			return nil, errors.New("analytics returned an invalid traffic breakdown item")
		}
		metrics, err := trafficMetrics(trafficMetricValues{
			requestCount:    item.GetRequestCount(),
			clientErrors:    item.GetClientErrorCount(),
			serverErrors:    item.GetServerErrorCount(),
			noResponses:     item.GetNoResponseCount(),
			averageDuration: item.GetAverageDuration(),
			p50Duration:     item.GetP50Duration(),
			p95Duration:     item.GetP95Duration(),
			p99Duration:     item.GetP99Duration(),
		})
		if err != nil {
			return nil, fmt.Errorf("convert traffic breakdown item: %w", err)
		}
		breakdown = append(breakdown, trafficbiz.BreakdownItem{ResourceID: item.GetResourceId(), Metrics: metrics})
	}
	return breakdown, nil
}

func resourceTrafficSummaries(items []*analyticsv1.ResourceTrafficSummary) ([]trafficbiz.ResourceTrafficSummary, error) {
	summaries := make([]trafficbiz.ResourceTrafficSummary, 0, len(items))
	for _, item := range items {
		if item == nil || item.GetResourceId() == "" {
			return nil, errors.New("analytics returned an invalid resource traffic summary")
		}
		summaries = append(summaries, trafficbiz.ResourceTrafficSummary{
			ResourceID:   item.GetResourceId(),
			RequestCount: item.GetRequestCount(),
			ServerErrors: item.GetServerErrorCount(),
			NoResponses:  item.GetNoResponseCount(),
		})
	}
	return summaries, nil
}

func trafficMetrics(value trafficMetricValues) (trafficbiz.Metrics, error) {
	classified := value.clientErrors + value.serverErrors + value.noResponses
	if classified > value.requestCount {
		return trafficbiz.Metrics{}, errors.New("analytics traffic counts are inconsistent")
	}
	averageDuration, err := analyticsDuration(value.averageDuration)
	if err != nil {
		return trafficbiz.Metrics{}, fmt.Errorf("invalid average duration: %w", err)
	}
	p50Duration, err := analyticsDuration(value.p50Duration)
	if err != nil {
		return trafficbiz.Metrics{}, fmt.Errorf("invalid p50 duration: %w", err)
	}
	p95Duration, err := analyticsDuration(value.p95Duration)
	if err != nil {
		return trafficbiz.Metrics{}, fmt.Errorf("invalid p95 duration: %w", err)
	}
	p99Duration, err := analyticsDuration(value.p99Duration)
	if err != nil {
		return trafficbiz.Metrics{}, fmt.Errorf("invalid p99 duration: %w", err)
	}
	return trafficbiz.Metrics{
		RequestCount:    value.requestCount,
		NonErrorCount:   value.requestCount - classified,
		ClientErrors:    value.clientErrors,
		ServerErrors:    value.serverErrors,
		NoResponses:     value.noResponses,
		AverageDuration: averageDuration,
		P50Duration:     p50Duration,
		P95Duration:     p95Duration,
		P99Duration:     p99Duration,
	}, nil
}

func analyticsDuration(value *durationpb.Duration) (time.Duration, error) {
	if value == nil {
		return 0, nil
	}
	if err := value.CheckValid(); err != nil {
		return 0, err
	}
	return value.AsDuration(), nil
}

func analyticsTimeBucket(value trafficbiz.TimeBucket) analyticsv1.TimeBucket {
	switch value {
	case trafficbiz.TimeBucketMinute:
		return analyticsv1.TimeBucket_TIME_BUCKET_MINUTE
	case trafficbiz.TimeBucketFiveMinutes:
		return analyticsv1.TimeBucket_TIME_BUCKET_FIVE_MINUTES
	case trafficbiz.TimeBucketHour:
		return analyticsv1.TimeBucket_TIME_BUCKET_HOUR
	default:
		return analyticsv1.TimeBucket_TIME_BUCKET_DAY
	}
}

func analyticsTrafficDimension(value trafficbiz.Dimension) analyticsv1.TrafficDimension {
	switch value {
	case trafficbiz.DimensionGateway:
		return analyticsv1.TrafficDimension_TRAFFIC_DIMENSION_GATEWAY
	case trafficbiz.DimensionService:
		return analyticsv1.TrafficDimension_TRAFFIC_DIMENSION_UPSTREAM
	default:
		return analyticsv1.TrafficDimension_TRAFFIC_DIMENSION_ROUTE
	}
}

func analyticsTrafficBreakdownOrder(value trafficbiz.BreakdownOrder) analyticsv1.TrafficBreakdownOrder {
	switch value {
	case trafficbiz.BreakdownOrderServerErrorRate:
		return analyticsv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_SERVER_ERROR_RATE
	case trafficbiz.BreakdownOrderP95Duration:
		return analyticsv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_P95_DURATION
	default:
		return analyticsv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_REQUEST_COUNT
	}
}
