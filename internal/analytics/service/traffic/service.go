// Package traffic 实现流量和延迟聚合查询协议
package traffic

import (
	"context"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

// Service 实现流量分析查询 API
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
	points, err := s.queries.Trend(ctx, query)
	if err != nil {
		return nil, err
	}
	response := &analyticsv1.GetTrafficTrendResponse{
		Points: make([]*analyticsv1.TrafficTrendPoint, 0, len(points)),
	}
	for _, point := range points {
		response.Points = append(response.Points, &analyticsv1.TrafficTrendPoint{
			StartedAt:        timestamppb.New(point.StartedAt),
			RequestCount:     point.RequestCount,
			ClientErrorCount: point.ClientErrors,
			ServerErrorCount: point.ServerErrors,
			AverageDuration:  durationpb.New(point.AverageDuration),
			P95Duration:      durationpb.New(point.P95Duration),
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
			AverageDuration:  durationpb.New(item.AverageDuration),
			P95Duration:      durationpb.New(item.P95Duration),
		})
	}
	return response, nil
}
