package traffic

import (
	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

func trendResponse(result trafficbiz.TrendResult) *analyticsv1.GetTrafficTrendResponse {
	return &analyticsv1.GetTrafficTrendResponse{
		Points: lo.Map(result.Points, func(point trafficbiz.TrendPoint, _ int) *analyticsv1.TrafficTrendPoint {
			return &analyticsv1.TrafficTrendPoint{
				StartedAt:        timestamppb.New(point.StartedAt),
				RequestCount:     point.RequestCount,
				ClientErrorCount: point.ClientErrorCount,
				ServerErrorCount: point.ServerErrorCount,
				NoResponseCount:  point.NoResponseCount,
				AverageDuration:  durationpb.New(point.AverageDuration),
				P50Duration:      durationpb.New(point.P50Duration),
				P95Duration:      durationpb.New(point.P95Duration),
				P99Duration:      durationpb.New(point.P99Duration),
			}
		}),
		Summary: summaryResponse(result.Summary),
	}
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
	return &analyticsv1.ListTrafficBreakdownResponse{
		Items: lo.Map(items, func(item trafficbiz.BreakdownItem, _ int) *analyticsv1.TrafficBreakdownItem {
			return &analyticsv1.TrafficBreakdownItem{
				ResourceId:       item.ResourceID,
				RequestCount:     item.RequestCount,
				ClientErrorCount: item.ClientErrorCount,
				ServerErrorCount: item.ServerErrorCount,
				NoResponseCount:  item.NoResponseCount,
				AverageDuration:  durationpb.New(item.AverageDuration),
				P50Duration:      durationpb.New(item.P50Duration),
				P95Duration:      durationpb.New(item.P95Duration),
				P99Duration:      durationpb.New(item.P99Duration),
			}
		}),
	}
}

func resourceTrafficResponse(
	summaries []trafficbiz.ResourceTrafficSummary,
) *analyticsv1.BatchGetResourceTrafficResponse {
	return &analyticsv1.BatchGetResourceTrafficResponse{
		Summaries: lo.Map(summaries, func(summary trafficbiz.ResourceTrafficSummary, _ int) *analyticsv1.ResourceTrafficSummary {
			return &analyticsv1.ResourceTrafficSummary{
				ResourceId:       summary.ResourceID,
				RequestCount:     summary.RequestCount,
				ServerErrorCount: summary.ServerErrorCount,
				NoResponseCount:  summary.NoResponseCount,
			}
		}),
	}
}
