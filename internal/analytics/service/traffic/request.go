package traffic

import (
	"slices"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/analytics/biz/traffic"
	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

const (
	defaultBreakdown uint32 = 20
	maxBreakdown     uint32 = 200

	maxResourceTrafficBatch = 200
)

func buildTrendQuery(request *analyticsv1.GetTrafficTrendRequest) (trafficbiz.TrendQuery, error) {
	filter, err := buildFilter(request.GetFilter())
	if err != nil {
		return trafficbiz.TrendQuery{}, err
	}
	bucket, err := buildTimeBucket(request.GetBucket())
	if err != nil {
		return trafficbiz.TrendQuery{}, err
	}
	return trafficbiz.TrendQuery{Filter: filter, Bucket: bucket}, nil
}

func buildBreakdownQuery(request *analyticsv1.ListTrafficBreakdownRequest) (trafficbiz.BreakdownQuery, error) {
	filter, err := buildFilter(request.GetFilter())
	if err != nil {
		return trafficbiz.BreakdownQuery{}, err
	}
	dimension, err := buildDimension(request.GetDimension())
	if err != nil {
		return trafficbiz.BreakdownQuery{}, err
	}
	order, err := buildBreakdownOrder(request.GetOrder())
	if err != nil {
		return trafficbiz.BreakdownQuery{}, err
	}
	limit := request.GetLimit()
	if limit == 0 {
		limit = defaultBreakdown
	}
	if limit > maxBreakdown {
		return trafficbiz.BreakdownQuery{}, invalidArgument("limit exceeds maximum")
	}
	return trafficbiz.BreakdownQuery{Filter: filter, Dimension: dimension, Order: order, Limit: int(limit)}, nil
}

func buildResourceTrafficQuery(
	request *analyticsv1.BatchGetResourceTrafficRequest,
) (trafficbiz.ResourceTrafficQuery, error) {
	startTime, endTime, err := buildTimeRange(request.GetStartTime(), request.GetEndTime())
	if err != nil {
		return trafficbiz.ResourceTrafficQuery{}, err
	}
	dimension, err := buildDimension(request.GetDimension())
	if err != nil {
		return trafficbiz.ResourceTrafficQuery{}, err
	}
	resourceIDs := request.GetResourceIds()
	if len(resourceIDs) == 0 {
		return trafficbiz.ResourceTrafficQuery{}, invalidArgument("resource_ids is required")
	}
	if len(resourceIDs) > maxResourceTrafficBatch {
		return trafficbiz.ResourceTrafficQuery{}, invalidArgument("resource_ids exceeds maximum")
	}
	seen := make(map[string]bool, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		if !resourceconfig.IsCanonicalID(resourceID) || seen[resourceID] {
			return trafficbiz.ResourceTrafficQuery{}, invalidArgument(
				"resource_ids contains an invalid or duplicate value",
			)
		}
		seen[resourceID] = true
	}
	return trafficbiz.ResourceTrafficQuery{
		StartTime:   startTime,
		EndTime:     endTime,
		Dimension:   dimension,
		ResourceIDs: slices.Clone(resourceIDs),
	}, nil
}

// 分钟聚合表无法准确表达分钟内的局部时间范围，因此协议边界必须对齐到整分钟
func buildFilter(filter *analyticsv1.TrafficFilter) (trafficbiz.Filter, error) {
	startTime, endTime, err := buildTimeRange(filter.GetStartTime(), filter.GetEndTime())
	if err != nil {
		return trafficbiz.Filter{}, err
	}
	for _, resourceID := range []string{
		filter.GetGatewayId(),
		filter.GetRouteId(),
		filter.GetUpstreamId(),
	} {
		if resourceID != "" && !resourceconfig.IsCanonicalID(resourceID) {
			return trafficbiz.Filter{}, invalidArgument("filter contains an invalid resource ID")
		}
	}
	return trafficbiz.Filter{
		StartTime:  startTime,
		EndTime:    endTime,
		GatewayID:  filter.GetGatewayId(),
		RouteID:    filter.GetRouteId(),
		UpstreamID: filter.GetUpstreamId(),
	}, nil
}

func buildTimeRange(start, end *timestamppb.Timestamp) (time.Time, time.Time, error) {
	if start == nil || end == nil || start.CheckValid() != nil || end.CheckValid() != nil {
		return time.Time{}, time.Time{}, invalidArgument("start_time and end_time are required")
	}
	startTime := start.AsTime()
	endTime := end.AsTime()
	if !analyticsconfig.IsValidQueryRange(startTime, endTime) {
		return time.Time{}, time.Time{}, invalidArgument("time range is invalid or exceeds the maximum")
	}
	if !startTime.Equal(startTime.Truncate(time.Minute)) ||
		!endTime.Equal(endTime.Truncate(time.Minute)) {
		return time.Time{}, time.Time{}, invalidArgument("start_time and end_time must align to minute boundaries")
	}
	return startTime, endTime, nil
}

func buildTimeBucket(value analyticsv1.TimeBucket) (trafficbiz.TimeBucket, error) {
	switch value {
	case analyticsv1.TimeBucket_TIME_BUCKET_MINUTE:
		return trafficbiz.TimeBucketMinute, nil
	case analyticsv1.TimeBucket_TIME_BUCKET_FIVE_MINUTES:
		return trafficbiz.TimeBucketFiveMinutes, nil
	case analyticsv1.TimeBucket_TIME_BUCKET_HOUR:
		return trafficbiz.TimeBucketHour, nil
	case analyticsv1.TimeBucket_TIME_BUCKET_DAY:
		return trafficbiz.TimeBucketDay, nil
	default:
		return 0, invalidArgument("bucket is invalid")
	}
}

func buildDimension(value analyticsv1.TrafficDimension) (trafficbiz.Dimension, error) {
	switch value {
	case analyticsv1.TrafficDimension_TRAFFIC_DIMENSION_GATEWAY:
		return trafficbiz.DimensionGateway, nil
	case analyticsv1.TrafficDimension_TRAFFIC_DIMENSION_ROUTE:
		return trafficbiz.DimensionRoute, nil
	case analyticsv1.TrafficDimension_TRAFFIC_DIMENSION_UPSTREAM:
		return trafficbiz.DimensionUpstream, nil
	default:
		return 0, invalidArgument("dimension is invalid")
	}
}

func buildBreakdownOrder(value analyticsv1.TrafficBreakdownOrder) (trafficbiz.BreakdownOrder, error) {
	switch value {
	case analyticsv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_UNSPECIFIED,
		analyticsv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_REQUEST_COUNT:
		return trafficbiz.BreakdownOrderRequestCount, nil
	case analyticsv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_SERVER_ERROR_RATE:
		return trafficbiz.BreakdownOrderServerErrorRate, nil
	case analyticsv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_P95_DURATION:
		return trafficbiz.BreakdownOrderP95Duration, nil
	default:
		return 0, invalidArgument("order is invalid")
	}
}

func invalidArgument(message string) error {
	return errors.BadRequest("INVALID_ARGUMENT", message)
}
