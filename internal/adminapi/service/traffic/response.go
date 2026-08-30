package traffic

import (
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
)

func analysisResponse(analysis trafficbiz.Analysis) *adminv1.GetTrafficAnalysisResponse {
	response := &adminv1.GetTrafficAnalysisResponse{
		Summary:            metricsResponse(analysis.Summary),
		BreakdownDimension: dimensionResponse(analysis.Dimension),
		BreakdownOrder:     orderResponse(analysis.Order),
		Trend:              make([]*adminv1.TrafficAnalysisPoint, len(analysis.Trend)),
		Breakdown:          make([]*adminv1.TrafficBreakdownItem, len(analysis.Breakdown)),
	}
	for i, point := range analysis.Trend {
		response.Trend[i] = &adminv1.TrafficAnalysisPoint{
			StartedAt: timestamppb.New(point.StartedAt),
			Metrics:   metricsResponse(point.Metrics),
		}
	}
	for i, breakdown := range analysis.Breakdown {
		response.Breakdown[i] = &adminv1.TrafficBreakdownItem{
			ResourceId: breakdown.ResourceID,
			Metrics:    metricsResponse(breakdown.Metrics),
		}
	}
	return response
}

func resourceTrafficResponse(summaries []trafficbiz.ResourceTrafficSummary) *adminv1.BatchGetResourceTrafficResponse {
	response := &adminv1.BatchGetResourceTrafficResponse{
		Summaries: make([]*adminv1.ResourceTrafficSummary, len(summaries)),
	}
	for i, summary := range summaries {
		response.Summaries[i] = &adminv1.ResourceTrafficSummary{
			ResourceId:       summary.ResourceID,
			RequestCount:     summary.RequestCount,
			ServerErrorCount: summary.ServerErrorCount,
			NoResponseCount:  summary.NoResponseCount,
		}
	}
	return response
}

func metricsResponse(metrics trafficbiz.Metrics) *adminv1.TrafficMetrics {
	return &adminv1.TrafficMetrics{
		RequestCount:     metrics.RequestCount,
		NonErrorCount:    metrics.NonErrorCount,
		ClientErrorCount: metrics.ClientErrorCount,
		ServerErrorCount: metrics.ServerErrorCount,
		NoResponseCount:  metrics.NoResponseCount,
		AverageDuration:  durationpb.New(metrics.AverageDuration),
		P50Duration:      durationpb.New(metrics.P50Duration),
		P95Duration:      durationpb.New(metrics.P95Duration),
		P99Duration:      durationpb.New(metrics.P99Duration),
	}
}

func dimensionResponse(value trafficbiz.Dimension) adminv1.TrafficBreakdownDimension {
	switch value {
	case trafficbiz.DimensionGateway:
		return adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY
	case trafficbiz.DimensionService:
		return adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_SERVICE
	default:
		return adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_ROUTE
	}
}

func orderResponse(value trafficbiz.BreakdownOrder) adminv1.TrafficBreakdownOrder {
	switch value {
	case trafficbiz.BreakdownOrderServerErrorRate:
		return adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_SERVER_ERROR_RATE
	case trafficbiz.BreakdownOrderP95Duration:
		return adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_P95_DURATION
	default:
		return adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_REQUEST_COUNT
	}
}
