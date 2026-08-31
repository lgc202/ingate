package traffic

import (
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

func trendResponse(result trafficbiz.TrendResult) *analyticsv1.GetTrafficTrendResponse {
	response := &analyticsv1.GetTrafficTrendResponse{
		Points:  make([]*analyticsv1.TrafficTrendPoint, 0, len(result.Points)),
		Summary: summaryResponse(result.Summary),
	}
	for _, point := range result.Points {
		response.Points = append(response.Points, &analyticsv1.TrafficTrendPoint{
			StartedAt:        timestamppb.New(point.StartedAt),
			RequestCount:     point.RequestCount,
			ClientErrorCount: point.ClientErrorCount,
			ServerErrorCount: point.ServerErrorCount,
			NoResponseCount:  point.NoResponseCount,
			AverageDuration:  durationpb.New(point.AverageDuration),
			P50Duration:      durationpb.New(point.P50Duration),
			P95Duration:      durationpb.New(point.P95Duration),
			P99Duration:      durationpb.New(point.P99Duration),
		})
	}
	return response
}

func summaryResponse(summary trafficbiz.Summary) *analyticsv1.TrafficSummary {
	return &analyticsv1.TrafficSummary{
		RequestCount:     summary.RequestCount,
		ClientErrorCount: summary.ClientErrorCount,
		ServerErrorCount: summary.ServerErrorCount,
		NoResponseCount:  summary.NoResponseCount,
		AverageDuration:  durationpb.New(summary.AverageDuration),
		P50Duration:      durationpb.New(summary.P50Duration),
		P95Duration:      durationpb.New(summary.P95Duration),
		P99Duration:      durationpb.New(summary.P99Duration),
	}
}

func breakdownResponse(items []trafficbiz.BreakdownItem) *analyticsv1.ListTrafficBreakdownResponse {
	response := &analyticsv1.ListTrafficBreakdownResponse{
		Items: make([]*analyticsv1.TrafficBreakdownItem, 0, len(items)),
	}
	for _, item := range items {
		response.Items = append(response.Items, &analyticsv1.TrafficBreakdownItem{
			ResourceId:       item.ResourceID,
			RequestCount:     item.RequestCount,
			ClientErrorCount: item.ClientErrorCount,
			ServerErrorCount: item.ServerErrorCount,
			NoResponseCount:  item.NoResponseCount,
			AverageDuration:  durationpb.New(item.AverageDuration),
			P50Duration:      durationpb.New(item.P50Duration),
			P95Duration:      durationpb.New(item.P95Duration),
			P99Duration:      durationpb.New(item.P99Duration),
		})
	}
	return response
}

func resourceTrafficResponse(
	summaries []trafficbiz.ResourceTrafficSummary,
) *analyticsv1.BatchGetResourceTrafficResponse {
	response := &analyticsv1.BatchGetResourceTrafficResponse{
		Summaries: make([]*analyticsv1.ResourceTrafficSummary, 0, len(summaries)),
	}
	for _, summary := range summaries {
		response.Summaries = append(response.Summaries, &analyticsv1.ResourceTrafficSummary{
			ResourceId:       summary.ResourceID,
			RequestCount:     summary.RequestCount,
			ServerErrorCount: summary.ServerErrorCount,
			NoResponseCount:  summary.NoResponseCount,
		})
	}
	return response
}
