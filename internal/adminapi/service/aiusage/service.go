// Package aiusage 提供控制台模型调用与 Token 用量分析 API。
package aiusage

import (
	"context"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	aiusagebiz "github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
)

// Service 实现模型用量分析查询 API。
type Service struct {
	aiUsage *aiusagebiz.Usecase
}

// NewService 创建模型用量分析协议服务。
func NewService(aiUsage *aiusagebiz.Usecase) *Service {
	return &Service{aiUsage: aiUsage}
}

// GetAIUsageAnalysis 查询同一范围内的模型用量汇总、趋势与排名。
func (s *Service) GetAIUsageAnalysis(
	ctx context.Context,
	request *adminv1.GetAIUsageAnalysisRequest,
) (*adminv1.GetAIUsageAnalysisResponse, error) {
	query, err := analysisQuery(request)
	if err != nil {
		return nil, err
	}
	analysis, err := s.aiUsage.Analyze(ctx, query)
	if err != nil {
		return nil, err
	}
	return analysisResponse(analysis), nil
}
