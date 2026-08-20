// Package traffic 实现流量和延迟聚合查询的 gRPC 协议转换
//
// 该层把公共时间粒度和资源维度转换为 biz 类型，不拼接 ClickHouse 查询
package traffic

import (
	"context"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

// Service 实现 Analytics TrafficService gRPC API
type Service struct {
	queries *trafficbiz.Queries
}

// NewService 创建流量分析查询服务
func NewService(queries *trafficbiz.Queries) *Service {
	return &Service{queries: queries}
}

// GetTrafficTrend 查询流量和延迟趋势
func (s *Service) GetTrafficTrend(
	ctx context.Context,
	request *analyticsv1.GetTrafficTrendRequest,
) (*analyticsv1.GetTrafficTrendResponse, error) {
	query, err := buildTrendQuery(request)
	if err != nil {
		return nil, err
	}
	summary, err := s.queries.Summary(ctx, query.Filter)
	if err != nil {
		return nil, err
	}
	points, err := s.queries.Trend(ctx, query)
	if err != nil {
		return nil, err
	}
	response := &analyticsv1.GetTrafficTrendResponse{
		Points: make([]*analyticsv1.TrafficTrendPoint, 0, len(points)),
		Summary: &analyticsv1.TrafficSummary{
			RequestCount:     summary.RequestCount,
			ClientErrorCount: summary.ClientErrors,
			ServerErrorCount: summary.ServerErrors,
			NoResponseCount:  summary.NoResponses,
			AverageDuration:  durationpb.New(summary.AverageDuration),
			P50Duration:      durationpb.New(summary.P50Duration),
			P95Duration:      durationpb.New(summary.P95Duration),
			P99Duration:      durationpb.New(summary.P99Duration),
		},
	}
	for _, point := range points {
		response.Points = append(response.Points, &analyticsv1.TrafficTrendPoint{
			StartedAt:        timestamppb.New(point.StartedAt),
			RequestCount:     point.RequestCount,
			ClientErrorCount: point.ClientErrors,
			ServerErrorCount: point.ServerErrors,
			NoResponseCount:  point.NoResponses,
			AverageDuration:  durationpb.New(point.AverageDuration),
			P50Duration:      durationpb.New(point.P50Duration),
			P95Duration:      durationpb.New(point.P95Duration),
			P99Duration:      durationpb.New(point.P99Duration),
		})
	}
	return response, nil
}

// ListTrafficBreakdown 查询资源维度的流量和延迟分布
func (s *Service) ListTrafficBreakdown(
	ctx context.Context,
	request *analyticsv1.ListTrafficBreakdownRequest,
) (*analyticsv1.ListTrafficBreakdownResponse, error) {
	query, err := buildBreakdownQuery(request)
	if err != nil {
		return nil, err
	}
	items, err := s.queries.Breakdown(ctx, query)
	if err != nil {
		return nil, err
	}
	response := &analyticsv1.ListTrafficBreakdownResponse{
		Items: make([]*analyticsv1.TrafficBreakdownItem, 0, len(items)),
	}
	for _, item := range items {
		response.Items = append(response.Items, &analyticsv1.TrafficBreakdownItem{
			ResourceId:       item.ResourceID,
			RequestCount:     item.RequestCount,
			ClientErrorCount: item.ClientErrors,
			ServerErrorCount: item.ServerErrors,
			NoResponseCount:  item.NoResponses,
			AverageDuration:  durationpb.New(item.AverageDuration),
			P50Duration:      durationpb.New(item.P50Duration),
			P95Duration:      durationpb.New(item.P95Duration),
			P99Duration:      durationpb.New(item.P99Duration),
		})
	}
	return response, nil
}

// BatchGetResourceTraffic 查询指定资源的列表流量摘要
func (s *Service) BatchGetResourceTraffic(
	ctx context.Context,
	request *analyticsv1.BatchGetResourceTrafficRequest,
) (*analyticsv1.BatchGetResourceTrafficResponse, error) {
	query, err := buildResourceTrafficQuery(request)
	if err != nil {
		return nil, err
	}
	summaries, err := s.queries.ResourceTraffic(ctx, query)
	if err != nil {
		return nil, err
	}
	response := &analyticsv1.BatchGetResourceTrafficResponse{
		Summaries: make([]*analyticsv1.ResourceTrafficSummary, 0, len(summaries)),
	}
	for _, summary := range summaries {
		response.Summaries = append(response.Summaries, &analyticsv1.ResourceTrafficSummary{
			ResourceId:       summary.ResourceID,
			RequestCount:     summary.RequestCount,
			ServerErrorCount: summary.ServerErrors,
			NoResponseCount:  summary.NoResponses,
		})
	}
	return response, nil
}
