package aiusage

import (
	"cmp"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	aiusagebiz "github.com/lgc202/ingate/internal/analytics/biz/aiusage"
	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

const (
	defaultBreakdownLimit uint32 = 20
	maxBreakdownLimit     uint32 = 200
)

func buildTrendQuery(request *analyticsv1.GetAIUsageTrendRequest) (aiusagebiz.TrendQuery, error) {
	filter, err := buildFilter(request.GetFilter())
	if err != nil {
		return aiusagebiz.TrendQuery{}, err
	}
	bucket, err := buildTimeBucket(request.GetBucket())
	if err != nil {
		return aiusagebiz.TrendQuery{}, err
	}
	return aiusagebiz.TrendQuery{Filter: filter, Bucket: bucket}, nil
}

func buildBreakdownQuery(request *analyticsv1.ListAIUsageBreakdownRequest) (aiusagebiz.BreakdownQuery, error) {
	filter, err := buildFilter(request.GetFilter())
	if err != nil {
		return aiusagebiz.BreakdownQuery{}, err
	}
	dimension, err := buildDimension(request.GetDimension())
	if err != nil {
		return aiusagebiz.BreakdownQuery{}, err
	}
	order, err := buildBreakdownOrder(request.GetOrder())
	if err != nil {
		return aiusagebiz.BreakdownQuery{}, err
	}
	limit := cmp.Or(request.GetLimit(), defaultBreakdownLimit)
	if limit > maxBreakdownLimit {
		return aiusagebiz.BreakdownQuery{}, invalidArgument("limit exceeds maximum")
	}
	return aiusagebiz.BreakdownQuery{Filter: filter, Dimension: dimension, Order: order, Limit: int(limit)}, nil
}

func buildFilter(filter *analyticsv1.AIUsageFilter) (aiusagebiz.Filter, error) {
	startTime, endTime, err := buildTimeRange(filter.GetStartTime(), filter.GetEndTime())
	if err != nil {
		return aiusagebiz.Filter{}, err
	}
	for _, resourceID := range []string{
		filter.GetGatewayId(),
		filter.GetCallerId(),
		filter.GetRouteId(),
		filter.GetUpstreamId(),
	} {
		if resourceID != "" && !resourceconfig.IsCanonicalID(resourceID) {
			return aiusagebiz.Filter{}, invalidArgument("filter contains an invalid resource ID")
		}
	}
	for _, model := range []string{filter.GetClientModel(), filter.GetUpstreamModel()} {
		if model != "" && !routeconfig.IsValidModelName(model) {
			return aiusagebiz.Filter{}, invalidArgument("filter contains an invalid model name")
		}
	}
	return aiusagebiz.Filter{
		StartTime:     startTime,
		EndTime:       endTime,
		GatewayID:     filter.GetGatewayId(),
		CallerID:      filter.GetCallerId(),
		RouteID:       filter.GetRouteId(),
		ClientModel:   filter.GetClientModel(),
		UpstreamID:    filter.GetUpstreamId(),
		UpstreamModel: filter.GetUpstreamModel(),
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

func buildTimeBucket(value analyticsv1.TimeBucket) (aiusagebiz.TimeBucket, error) {
	switch value {
	case analyticsv1.TimeBucket_TIME_BUCKET_MINUTE:
		return aiusagebiz.TimeBucketMinute, nil
	case analyticsv1.TimeBucket_TIME_BUCKET_FIVE_MINUTES:
		return aiusagebiz.TimeBucketFiveMinutes, nil
	case analyticsv1.TimeBucket_TIME_BUCKET_HOUR:
		return aiusagebiz.TimeBucketHour, nil
	case analyticsv1.TimeBucket_TIME_BUCKET_DAY:
		return aiusagebiz.TimeBucketDay, nil
	default:
		return 0, invalidArgument("bucket is invalid")
	}
}

func buildDimension(value analyticsv1.AIUsageDimension) (aiusagebiz.Dimension, error) {
	switch value {
	case analyticsv1.AIUsageDimension_AI_USAGE_DIMENSION_CALLER:
		return aiusagebiz.DimensionCaller, nil
	case analyticsv1.AIUsageDimension_AI_USAGE_DIMENSION_ROUTE:
		return aiusagebiz.DimensionRoute, nil
	case analyticsv1.AIUsageDimension_AI_USAGE_DIMENSION_CLIENT_MODEL:
		return aiusagebiz.DimensionClientModel, nil
	case analyticsv1.AIUsageDimension_AI_USAGE_DIMENSION_UPSTREAM:
		return aiusagebiz.DimensionUpstream, nil
	case analyticsv1.AIUsageDimension_AI_USAGE_DIMENSION_UPSTREAM_MODEL:
		return aiusagebiz.DimensionUpstreamModel, nil
	default:
		return 0, invalidArgument("dimension is invalid")
	}
}

func buildBreakdownOrder(value analyticsv1.AIUsageBreakdownOrder) (aiusagebiz.BreakdownOrder, error) {
	switch value {
	case analyticsv1.AIUsageBreakdownOrder_AI_USAGE_BREAKDOWN_ORDER_UNSPECIFIED,
		analyticsv1.AIUsageBreakdownOrder_AI_USAGE_BREAKDOWN_ORDER_CALL_COUNT:
		return aiusagebiz.BreakdownOrderCallCount, nil
	case analyticsv1.AIUsageBreakdownOrder_AI_USAGE_BREAKDOWN_ORDER_TOTAL_TOKENS:
		return aiusagebiz.BreakdownOrderTotalTokens, nil
	default:
		return 0, invalidArgument("order is invalid")
	}
}

func invalidArgument(message string) error {
	return errors.BadRequest("INVALID_ARGUMENT", message)
}
