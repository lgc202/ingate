// Package traffic 提供控制台流量分析 API。
package traffic

import (
	"context"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
)

// Service 实现流量分析查询 API。
type Service struct {
	traffic *trafficbiz.Usecase
}

// NewService 创建流量分析协议服务。
func NewService(traffic *trafficbiz.Usecase) *Service {
	return &Service{traffic: traffic}
}

// GetTrafficAnalysis 查询同一范围内的汇总、趋势和资源排名。
func (s *Service) GetTrafficAnalysis(
	ctx context.Context,
	request *adminv1.GetTrafficAnalysisRequest,
) (*adminv1.GetTrafficAnalysisResponse, error) {
	query, err := analysisQuery(request)
	if err != nil {
		return nil, err
	}
	analysis, err := s.traffic.Analyze(ctx, query)
	if err != nil {
		return nil, err
	}
	return analysisResponse(analysis), nil
}

// BatchGetResourceTraffic 查询资源列表展示所需的最近流量摘要。
func (s *Service) BatchGetResourceTraffic(
	ctx context.Context,
	request *adminv1.BatchGetResourceTrafficRequest,
) (*adminv1.BatchGetResourceTrafficResponse, error) {
	query, err := resourceTrafficQuery(request)
	if err != nil {
		return nil, err
	}
	summaries, err := s.traffic.BatchGetResourceTraffic(ctx, query)
	if err != nil {
		return nil, err
	}
	return resourceTrafficResponse(summaries), nil
}
