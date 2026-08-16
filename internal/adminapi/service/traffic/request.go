package traffic

import (
	"time"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

const maximumTimeRange = 90 * 24 * time.Hour

func analysisQuery(request *adminv1.GetTrafficAnalysisRequest) (trafficbiz.Query, error) {
	start := request.GetStartTime()
	end := request.GetEndTime()
	if start == nil || start.CheckValid() != nil {
		return trafficbiz.Query{}, adminservice.BadRequest("请选择查询开始时间")
	}
	if end == nil || end.CheckValid() != nil {
		return trafficbiz.Query{}, adminservice.BadRequest("请选择查询结束时间")
	}
	startTime := start.AsTime()
	endTime := end.AsTime()
	if !startTime.Before(endTime) {
		return trafficbiz.Query{}, adminservice.BadRequest("查询开始时间必须早于结束时间")
	}
	if endTime.Sub(startTime) > maximumTimeRange {
		return trafficbiz.Query{}, adminservice.BadRequest("单次最多查询 90 天流量")
	}
	return trafficbiz.Query{
		Filter: trafficbiz.Filter{
			StartTime: alignStartTime(startTime),
			EndTime:   alignEndTime(endTime),
			GatewayID: request.GetGatewayId(),
			RouteID:   request.GetRouteId(),
			ServiceID: request.GetServiceId(),
		},
		Dimension: breakdownDimension(request.GetBreakdownDimension()),
		Order:     breakdownOrder(request.GetBreakdownOrder()),
		Limit:     int(request.GetBreakdownLimit()),
	}, nil
}

func alignStartTime(value time.Time) time.Time {
	return value.Truncate(time.Minute)
}

func alignEndTime(value time.Time) time.Time {
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
