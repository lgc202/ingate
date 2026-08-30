// Package traffic 实现流量和延迟聚合查询的 gRPC 协议转换。
//
// 该层把公共时间粒度和资源维度转换为 biz 类型，不拼接 ClickHouse 查询。
package traffic

import (
	"context"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

// Service 实现 Analytics TrafficService gRPC API。
type Service struct {
	query *trafficbiz.Query
}

// NewService 创建流量分析查询服务。
func NewService(query *trafficbiz.Query) *Service {
	return &Service{query: query}
}

// GetTrafficTrend 查询流量和延迟趋势。
func (s *Service) GetTrafficTrend(
	ctx context.Context,
	request *analyticsv1.GetTrafficTrendRequest,
) (*analyticsv1.GetTrafficTrendResponse, error) {
	query, err := buildTrendQuery(request)
	if err != nil {
		return nil, err
	}
	result, err := s.query.Trend(ctx, query)
	if err != nil {
		return nil, err
	}
	return trendResponse(result), nil
}

// ListTrafficBreakdown 查询资源维度的流量和延迟分布。
func (s *Service) ListTrafficBreakdown(
	ctx context.Context,
	request *analyticsv1.ListTrafficBreakdownRequest,
) (*analyticsv1.ListTrafficBreakdownResponse, error) {
	query, err := buildBreakdownQuery(request)
	if err != nil {
		return nil, err
	}
	items, err := s.query.Breakdown(ctx, query)
	if err != nil {
		return nil, err
	}
	return breakdownResponse(items), nil
}

// BatchGetResourceTraffic 查询指定资源的列表流量摘要。
func (s *Service) BatchGetResourceTraffic(
	ctx context.Context,
	request *analyticsv1.BatchGetResourceTrafficRequest,
) (*analyticsv1.BatchGetResourceTrafficResponse, error) {
	query, err := buildResourceTrafficQuery(request)
	if err != nil {
		return nil, err
	}
	summaries, err := s.query.ResourceTraffic(ctx, query)
	if err != nil {
		return nil, err
	}
	return resourceTrafficResponse(summaries), nil
}
