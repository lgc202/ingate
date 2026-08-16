package traffic

import (
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

const (
	defaultBreakdown = 20
	maxBreakdown     = 200
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
	limit := int(request.GetLimit())
	if limit == 0 {
		limit = defaultBreakdown
	}
	if limit > maxBreakdown {
		return trafficbiz.BreakdownQuery{}, invalidArgument("limit exceeds maximum")
	}
	return trafficbiz.BreakdownQuery{Filter: filter, Dimension: dimension, Order: order, Limit: limit}, nil
}

// 分钟聚合表无法准确表达分钟内的局部时间范围，因此协议边界必须对齐到整分钟
func buildFilter(filter *analyticsv1.TrafficFilter) (trafficbiz.Filter, error) {
	start := filter.GetStartTime()
	end := filter.GetEndTime()
	if start == nil || end == nil || start.CheckValid() != nil || end.CheckValid() != nil {
		return trafficbiz.Filter{}, invalidArgument("start_time and end_time are required")
	}
	if !start.AsTime().Before(end.AsTime()) {
		return trafficbiz.Filter{}, invalidArgument("start_time must be before end_time")
	}
	if !start.AsTime().Equal(start.AsTime().Truncate(time.Minute)) ||
		!end.AsTime().Equal(end.AsTime().Truncate(time.Minute)) {
		return trafficbiz.Filter{}, invalidArgument("start_time and end_time must align to minute boundaries")
	}
	return trafficbiz.Filter{
		StartTime:  start.AsTime(),
		EndTime:    end.AsTime(),
		GatewayID:  filter.GetGatewayId(),
		RouteID:    filter.GetRouteId(),
		UpstreamID: filter.GetUpstreamId(),
	}, nil
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
	return kratoserrors.BadRequest("INVALID_ARGUMENT", message)
}
