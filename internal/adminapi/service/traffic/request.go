package traffic

import (
	"slices"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

func analysisQuery(request *adminv1.GetTrafficAnalysisRequest) (trafficbiz.Query, error) {
	startTime, endTime, err := trafficTimeRange(request.GetStartTime(), request.GetEndTime())
	if err != nil {
		return trafficbiz.Query{}, err
	}
	for _, resourceID := range []string{
		request.GetGatewayId(),
		request.GetRouteId(),
		request.GetServiceId(),
	} {
		if resourceID != "" && !resourceconfig.IsCanonicalID(resourceID) {
			return trafficbiz.Query{}, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"资源筛选条件无效",
			)
		}
	}
	return trafficbiz.Query{
		Filter: trafficbiz.Filter{
			StartTime: startTime,
			EndTime:   endTime,
			GatewayID: request.GetGatewayId(),
			RouteID:   request.GetRouteId(),
			ServiceID: request.GetServiceId(),
		},
		Dimension: breakdownDimension(request.GetBreakdownDimension()),
		Order:     breakdownOrder(request.GetBreakdownOrder()),
		Limit:     int(request.GetBreakdownLimit()),
	}, nil
}

func resourceTrafficQuery(request *adminv1.BatchGetResourceTrafficRequest) (trafficbiz.ResourceTrafficQuery, error) {
	startTime, endTime, err := trafficTimeRange(request.GetStartTime(), request.GetEndTime())
	if err != nil {
		return trafficbiz.ResourceTrafficQuery{}, err
	}
	resourceIDs := request.GetResourceIds()
	seen := make(map[string]bool, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		if !resourceconfig.IsCanonicalID(resourceID) || seen[resourceID] {
			return trafficbiz.ResourceTrafficQuery{}, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"资源标识包含无效或重复值",
			)
		}
		seen[resourceID] = true
	}
	dimension, err := resourceTrafficDimension(request.GetDimension())
	if err != nil {
		return trafficbiz.ResourceTrafficQuery{}, err
	}
	return trafficbiz.ResourceTrafficQuery{
		StartTime:   startTime,
		EndTime:     endTime,
		Dimension:   dimension,
		ResourceIDs: slices.Clone(resourceIDs),
	}, nil
}

func trafficTimeRange(start, end *timestamppb.Timestamp) (time.Time, time.Time, error) {
	if start == nil || start.CheckValid() != nil {
		return time.Time{}, time.Time{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"请选择查询开始时间",
		)
	}
	if end == nil || end.CheckValid() != nil {
		return time.Time{}, time.Time{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"请选择查询结束时间",
		)
	}
	requestedStart := start.AsTime()
	requestedEnd := end.AsTime()
	if !requestedStart.Before(requestedEnd) {
		return time.Time{}, time.Time{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"查询开始时间必须早于结束时间",
		)
	}
	startTime := alignStartTime(requestedStart)
	endTime := alignEndTime(requestedEnd)
	if !analyticsconfig.IsValidQueryRange(startTime, endTime) {
		return time.Time{}, time.Time{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"单次最多查询 90 天流量",
		)
	}
	return startTime, endTime, nil
}

func alignStartTime(value time.Time) time.Time {
	return value.Truncate(time.Minute)
}

func alignEndTime(value time.Time) time.Time {
	// 聚合桶使用左闭右开区间，结束时间向上对齐才能覆盖最后一个不完整分钟。
	aligned := value.Truncate(time.Minute)
	if aligned.Equal(value) {
		return aligned
	}
	return aligned.Add(time.Minute)
}

func breakdownDimension(value adminv1.TrafficBreakdownDimension) trafficbiz.Dimension {
	switch value {
	case adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY:
		return trafficbiz.DimensionGateway
	case adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_SERVICE:
		return trafficbiz.DimensionService
	default:
		return trafficbiz.DimensionRoute
	}
}

func resourceTrafficDimension(value adminv1.TrafficBreakdownDimension) (trafficbiz.Dimension, error) {
	switch value {
	case adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY:
		return trafficbiz.DimensionGateway, nil
	case adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_ROUTE:
		return trafficbiz.DimensionRoute, nil
	case adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_SERVICE:
		return trafficbiz.DimensionService, nil
	default:
		return 0, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"请选择资源类型",
		)
	}
}

func breakdownOrder(value adminv1.TrafficBreakdownOrder) trafficbiz.BreakdownOrder {
	switch value {
	case adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_SERVER_ERROR_RATE:
		return trafficbiz.BreakdownOrderServerErrorRate
	case adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_P95_DURATION:
		return trafficbiz.BreakdownOrderP95Duration
	default:
		return trafficbiz.BreakdownOrderRequestCount
	}
}
