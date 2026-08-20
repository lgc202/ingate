package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
)

// TrafficRepository 通过 Analytics gRPC 查询流量聚合结果
type TrafficRepository struct {
	client analyticsv1.TrafficServiceClient
}

// NewTrafficRepository 创建流量分析 Repository
func NewTrafficRepository(connection *grpc.ClientConn) *TrafficRepository {
	return &TrafficRepository{client: analyticsv1.NewTrafficServiceClient(connection)}
}

// Analyze 查询同一范围内的汇总、趋势和资源排名
func (r *TrafficRepository) Analyze(ctx context.Context, query trafficbiz.Query) (trafficbiz.Analysis, error) {
	filter := &analyticsv1.TrafficFilter{
		StartTime:  timestamppb.New(query.Filter.StartTime),
		EndTime:    timestamppb.New(query.Filter.EndTime),
		GatewayId:  query.Filter.GatewayID,
		RouteId:    query.Filter.RouteID,
		UpstreamId: query.Filter.ServiceID,
	}
	trendReply, err := r.client.GetTrafficTrend(ctx, &analyticsv1.GetTrafficTrendRequest{
		Filter: filter,
		Bucket: analyticsTimeBucket(query.Bucket),
	})
	if err != nil {
		return trafficbiz.Analysis{}, trafficQueryError("query traffic trend", err)
	}
	breakdownReply, err := r.client.ListTrafficBreakdown(ctx, &analyticsv1.ListTrafficBreakdownRequest{
		Filter:    filter,
		Dimension: analyticsTrafficDimension(query.Dimension),
		Order:     analyticsTrafficBreakdownOrder(query.Order),
		Limit:     uint32(query.Limit),
	})
	if err != nil {
		return trafficbiz.Analysis{}, trafficQueryError("query traffic breakdown", err)
	}

	if trendReply.GetSummary() == nil {
		return trafficbiz.Analysis{}, errors.New("analytics returned an empty traffic summary")
	}
	summary, err := trafficMetrics(metricValuesFromSummary(trendReply.GetSummary()))
	if err != nil {
		return trafficbiz.Analysis{}, fmt.Errorf("convert traffic summary: %w", err)
	}
	trend := make([]trafficbiz.TrendPoint, 0, len(trendReply.GetPoints()))
	for _, point := range trendReply.GetPoints() {
		if point == nil || point.GetStartedAt() == nil || point.GetStartedAt().CheckValid() != nil {
			return trafficbiz.Analysis{}, errors.New("analytics returned an invalid traffic trend point")
		}
		metrics, err := trafficMetrics(metricValuesFromPoint(point))
		if err != nil {
			return trafficbiz.Analysis{}, fmt.Errorf("convert traffic trend point: %w", err)
		}
		trend = append(trend, trafficbiz.TrendPoint{StartedAt: point.GetStartedAt().AsTime(), Metrics: metrics})
	}
	breakdown := make([]trafficbiz.BreakdownItem, 0, len(breakdownReply.GetItems()))
	for _, item := range breakdownReply.GetItems() {
		if item == nil || item.GetResourceId() == "" {
			return trafficbiz.Analysis{}, errors.New("analytics returned an invalid traffic breakdown item")
		}
		metrics, err := trafficMetrics(metricValuesFromBreakdown(item))
		if err != nil {
			return trafficbiz.Analysis{}, fmt.Errorf("convert traffic breakdown item: %w", err)
		}
		breakdown = append(breakdown, trafficbiz.BreakdownItem{ResourceID: item.GetResourceId(), Metrics: metrics})
	}
	return trafficbiz.Analysis{
		Summary:   summary,
		Trend:     trend,
		Dimension: query.Dimension,
		Order:     query.Order,
		Breakdown: breakdown,
	}, nil
}

// BatchGetResourceTraffic 查询指定资源的列表流量摘要
func (r *TrafficRepository) BatchGetResourceTraffic(
	ctx context.Context,
	query trafficbiz.ResourceTrafficQuery,
) ([]trafficbiz.ResourceTrafficSummary, error) {
	reply, err := r.client.BatchGetResourceTraffic(ctx, &analyticsv1.BatchGetResourceTrafficRequest{
		StartTime:   timestamppb.New(query.StartTime),
		EndTime:     timestamppb.New(query.EndTime),
		Dimension:   analyticsTrafficDimension(query.Dimension),
		ResourceIds: query.ResourceIDs,
	})
	if err != nil {
		return nil, trafficQueryError("query resource traffic", err)
	}
	summaries := make([]trafficbiz.ResourceTrafficSummary, 0, len(reply.GetSummaries()))
	for _, summary := range reply.GetSummaries() {
		if summary == nil || summary.GetResourceId() == "" {
			return nil, errors.New("analytics returned an invalid resource traffic summary")
		}
		summaries = append(summaries, trafficbiz.ResourceTrafficSummary{
			ResourceID:   summary.GetResourceId(),
			RequestCount: summary.GetRequestCount(),
			ServerErrors: summary.GetServerErrorCount(),
			NoResponses:  summary.GetNoResponseCount(),
		})
	}
	return summaries, nil
}

type metricValues struct {
	requestCount    uint64
	clientErrors    uint64
	serverErrors    uint64
	noResponses     uint64
	averageDuration *durationpb.Duration
	p50Duration     *durationpb.Duration
	p95Duration     *durationpb.Duration
	p99Duration     *durationpb.Duration
}

func metricValuesFromSummary(summary *analyticsv1.TrafficSummary) metricValues {
	if summary == nil {
		return metricValues{}
	}
	return metricValues{
		requestCount:    summary.GetRequestCount(),
		clientErrors:    summary.GetClientErrorCount(),
		serverErrors:    summary.GetServerErrorCount(),
		noResponses:     summary.GetNoResponseCount(),
		averageDuration: summary.GetAverageDuration(),
		p50Duration:     summary.GetP50Duration(),
		p95Duration:     summary.GetP95Duration(),
		p99Duration:     summary.GetP99Duration(),
	}
}

func metricValuesFromPoint(point *analyticsv1.TrafficTrendPoint) metricValues {
	return metricValues{
		requestCount:    point.GetRequestCount(),
		clientErrors:    point.GetClientErrorCount(),
		serverErrors:    point.GetServerErrorCount(),
		noResponses:     point.GetNoResponseCount(),
		averageDuration: point.GetAverageDuration(),
		p50Duration:     point.GetP50Duration(),
		p95Duration:     point.GetP95Duration(),
		p99Duration:     point.GetP99Duration(),
	}
}

func metricValuesFromBreakdown(item *analyticsv1.TrafficBreakdownItem) metricValues {
	return metricValues{
		requestCount:    item.GetRequestCount(),
		clientErrors:    item.GetClientErrorCount(),
		serverErrors:    item.GetServerErrorCount(),
		noResponses:     item.GetNoResponseCount(),
		averageDuration: item.GetAverageDuration(),
		p50Duration:     item.GetP50Duration(),
		p95Duration:     item.GetP95Duration(),
		p99Duration:     item.GetP99Duration(),
	}
}

func trafficMetrics(value metricValues) (trafficbiz.Metrics, error) {
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

func trafficQueryError(operation string, err error) error {
	if status.Code(err) == codes.Unavailable || status.Code(err) == codes.DeadlineExceeded {
		return trafficbiz.Unavailable(err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
