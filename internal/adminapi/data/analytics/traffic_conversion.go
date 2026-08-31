package analytics

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
)

type trafficMetricValues struct {
	requestCount     uint64
	clientErrorCount uint64
	serverErrorCount uint64
	noResponseCount  uint64
	averageDuration  *durationpb.Duration
	p50Duration      *durationpb.Duration
	p95Duration      *durationpb.Duration
	p99Duration      *durationpb.Duration
}

func analyticsTrafficFilter(filter trafficbiz.Filter) *analyticsv1.TrafficFilter {
	return &analyticsv1.TrafficFilter{
		StartTime:  timestamppb.New(filter.StartTime),
		EndTime:    timestamppb.New(filter.EndTime),
		GatewayId:  filter.GatewayID,
		RouteId:    filter.RouteID,
		UpstreamId: filter.ServiceID,
	}
}

func trafficSummary(summary *analyticsv1.TrafficSummary) (trafficbiz.Metrics, error) {
	if summary == nil {
		return trafficbiz.Metrics{}, errors.New("analytics returned an empty traffic summary")
	}
	return trafficMetrics(trafficMetricValues{
		requestCount:     summary.GetRequestCount(),
		clientErrorCount: summary.GetClientErrorCount(),
		serverErrorCount: summary.GetServerErrorCount(),
		noResponseCount:  summary.GetNoResponseCount(),
		averageDuration:  summary.GetAverageDuration(),
		p50Duration:      summary.GetP50Duration(),
		p95Duration:      summary.GetP95Duration(),
		p99Duration:      summary.GetP99Duration(),
	})
}

func trafficTrend(
	points []*analyticsv1.TrafficTrendPoint,
	filter trafficbiz.Filter,
) ([]trafficbiz.TrendPoint, error) {
	trend := make([]trafficbiz.TrendPoint, len(points))
	var previousStart time.Time
	for i, point := range points {
		if point == nil || point.GetStartedAt() == nil || point.GetStartedAt().CheckValid() != nil {
			return nil, errors.New("analytics returned an invalid traffic trend point")
		}
		startedAt := point.GetStartedAt().AsTime()
		if startedAt.Before(filter.StartTime) || !startedAt.Before(filter.EndTime) {
			return nil, errors.New("analytics returned a traffic trend point outside the query range")
		}
		if i > 0 && !startedAt.After(previousStart) {
			return nil, errors.New("analytics returned unordered traffic trend points")
		}
		metrics, err := trafficMetrics(trafficMetricValues{
			requestCount:     point.GetRequestCount(),
			clientErrorCount: point.GetClientErrorCount(),
			serverErrorCount: point.GetServerErrorCount(),
			noResponseCount:  point.GetNoResponseCount(),
			averageDuration:  point.GetAverageDuration(),
			p50Duration:      point.GetP50Duration(),
			p95Duration:      point.GetP95Duration(),
			p99Duration:      point.GetP99Duration(),
		})
		if err != nil {
			return nil, fmt.Errorf("convert traffic trend point: %w", err)
		}
		trend[i] = trafficbiz.TrendPoint{
			StartedAt: startedAt,
			Metrics:   metrics,
		}
		previousStart = startedAt
	}
	return trend, nil
}

func trafficBreakdown(items []*analyticsv1.TrafficBreakdownItem) ([]trafficbiz.BreakdownItem, error) {
	breakdown := make([]trafficbiz.BreakdownItem, len(items))
	seen := make(map[string]bool, len(items))
	for i, item := range items {
		if item == nil || item.GetResourceId() == "" {
			return nil, errors.New("analytics returned an invalid traffic breakdown item")
		}
		resourceID := item.GetResourceId()
		if seen[resourceID] {
			return nil, errors.New("analytics returned duplicate traffic breakdown items")
		}
		seen[resourceID] = true
		metrics, err := trafficMetrics(trafficMetricValues{
			requestCount:     item.GetRequestCount(),
			clientErrorCount: item.GetClientErrorCount(),
			serverErrorCount: item.GetServerErrorCount(),
			noResponseCount:  item.GetNoResponseCount(),
			averageDuration:  item.GetAverageDuration(),
			p50Duration:      item.GetP50Duration(),
			p95Duration:      item.GetP95Duration(),
			p99Duration:      item.GetP99Duration(),
		})
		if err != nil {
			return nil, fmt.Errorf("convert traffic breakdown item: %w", err)
		}
		breakdown[i] = trafficbiz.BreakdownItem{
			ResourceID: resourceID,
			Metrics:    metrics,
		}
	}
	return breakdown, nil
}

func resourceTrafficSummaries(
	items []*analyticsv1.ResourceTrafficSummary,
	requestedIDs []string,
) ([]trafficbiz.ResourceTrafficSummary, error) {
	requested := make(map[string]bool, len(requestedIDs))
	for _, resourceID := range requestedIDs {
		requested[resourceID] = true
	}
	summaries := make([]trafficbiz.ResourceTrafficSummary, len(items))
	seen := make(map[string]bool, len(items))
	for i, item := range items {
		if item == nil || item.GetResourceId() == "" {
			return nil, errors.New("analytics returned an invalid resource traffic summary")
		}
		resourceID := item.GetResourceId()
		if !requested[resourceID] {
			return nil, errors.New("analytics returned resource traffic outside the requested set")
		}
		if seen[resourceID] {
			return nil, errors.New("analytics returned duplicate resource traffic summaries")
		}
		seen[resourceID] = true
		if item.GetServerErrorCount() > item.GetRequestCount() ||
			item.GetNoResponseCount() > item.GetRequestCount()-item.GetServerErrorCount() {
			return nil, errors.New("analytics resource traffic counts are inconsistent")
		}
		summaries[i] = trafficbiz.ResourceTrafficSummary{
			ResourceID:       resourceID,
			RequestCount:     item.GetRequestCount(),
			ServerErrorCount: item.GetServerErrorCount(),
			NoResponseCount:  item.GetNoResponseCount(),
		}
	}
	return summaries, nil
}

func trafficMetrics(value trafficMetricValues) (trafficbiz.Metrics, error) {
	nonErrorCount := value.requestCount
	if value.clientErrorCount > nonErrorCount {
		return trafficbiz.Metrics{}, errors.New("analytics traffic counts are inconsistent")
	}
	nonErrorCount -= value.clientErrorCount
	if value.serverErrorCount > nonErrorCount {
		return trafficbiz.Metrics{}, errors.New("analytics traffic counts are inconsistent")
	}
	nonErrorCount -= value.serverErrorCount
	if value.noResponseCount > nonErrorCount {
		return trafficbiz.Metrics{}, errors.New("analytics traffic counts are inconsistent")
	}
	nonErrorCount -= value.noResponseCount
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
		RequestCount:     value.requestCount,
		NonErrorCount:    nonErrorCount,
		ClientErrorCount: value.clientErrorCount,
		ServerErrorCount: value.serverErrorCount,
		NoResponseCount:  value.noResponseCount,
		AverageDuration:  averageDuration,
		P50Duration:      p50Duration,
		P95Duration:      p95Duration,
		P99Duration:      p99Duration,
	}, nil
}

func analyticsDuration(value *durationpb.Duration) (time.Duration, error) {
	if value == nil {
		return 0, nil
	}
	if err := value.CheckValid(); err != nil {
		return 0, err
	}
	duration := value.AsDuration()
	if duration < 0 {
		return 0, errors.New("duration must not be negative")
	}
	return duration, nil
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
