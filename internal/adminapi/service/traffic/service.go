// Package traffic 提供控制台流量分析 API
package traffic

import (
	"context"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
)

// Service 实现流量分析管理 API
type Service struct {
	business *trafficbiz.Service
}

// NewService 创建流量分析协议服务
func NewService(business *trafficbiz.Service) *Service {
	return &Service{business: business}
}

// GetTrafficAnalysis 查询同一范围内的汇总、趋势和资源排名
func (s *Service) GetTrafficAnalysis(
	ctx context.Context,
	request *adminv1.GetTrafficAnalysisRequest,
) (*adminv1.GetTrafficAnalysisResponse, error) {
	query, err := analysisQuery(request)
	if err != nil {
		return nil, err
	}
	analysis, err := s.business.Analyze(ctx, query)
	if err != nil {
		return nil, err
	}
	return analysisResponse(analysis), nil
}
