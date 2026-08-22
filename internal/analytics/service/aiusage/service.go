// Package aiusage 实现模型调用与 Token 用量查询的 gRPC 协议转换
package aiusage

import (
	"context"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	aiusagebiz "github.com/lgc202/ingate/internal/analytics/biz/aiusage"
)

// Service 实现 Analytics AIUsageService gRPC API
type Service struct {
	query *aiusagebiz.Query
}

// NewService 创建模型用量查询服务
func NewService(query *aiusagebiz.Query) *Service {
	return &Service{query: query}
}

// GetAIUsageTrend 查询模型调用和 Token 趋势
func (s *Service) GetAIUsageTrend(
	ctx context.Context,
	request *analyticsv1.GetAIUsageTrendRequest,
) (*analyticsv1.GetAIUsageTrendResponse, error) {
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

// ListAIUsageBreakdown 查询业务维度的模型调用和 Token 分布
func (s *Service) ListAIUsageBreakdown(
	ctx context.Context,
	request *analyticsv1.ListAIUsageBreakdownRequest,
) (*analyticsv1.ListAIUsageBreakdownResponse, error) {
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
