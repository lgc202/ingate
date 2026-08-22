package analytics

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	aiusagebiz "github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
)

// AIUsageRepository 通过 Analytics gRPC 查询模型调用与 Token 聚合结果
type AIUsageRepository struct {
	client analyticsv1.AIUsageServiceClient
}

// NewAIUsageRepository 创建模型用量分析 Repository
func NewAIUsageRepository(connection *grpc.ClientConn) *AIUsageRepository {
	return &AIUsageRepository{client: analyticsv1.NewAIUsageServiceClient(connection)}
}

// Analyze 查询同一范围内的模型用量汇总、趋势与排名
func (r *AIUsageRepository) Analyze(ctx context.Context, query aiusagebiz.Query) (aiusagebiz.Analysis, error) {
	filter := &analyticsv1.AIUsageFilter{
		StartTime:     timestamppb.New(query.Filter.StartTime),
		EndTime:       timestamppb.New(query.Filter.EndTime),
		GatewayId:     query.Filter.GatewayID,
		CallerId:      query.Filter.CallerID,
		RouteId:       query.Filter.RouteID,
		ClientModel:   query.Filter.ClientModel,
		UpstreamId:    query.Filter.ServiceID,
		UpstreamModel: query.Filter.ActualModel,
	}
	trendReply, err := r.client.GetAIUsageTrend(ctx, &analyticsv1.GetAIUsageTrendRequest{
		Filter: filter,
		Bucket: analyticsAIUsageTimeBucket(query.Bucket),
	})
	if err != nil {
		return aiusagebiz.Analysis{}, aiUsageQueryError("query AI usage trend", err)
	}
	breakdownReply, err := r.client.ListAIUsageBreakdown(ctx, &analyticsv1.ListAIUsageBreakdownRequest{
		Filter:    filter,
		Dimension: analyticsAIUsageDimension(query.Dimension),
		Order:     analyticsAIUsageBreakdownOrder(query.Order),
		Limit:     uint32(query.Limit),
	})
	if err != nil {
		return aiusagebiz.Analysis{}, aiUsageQueryError("query AI usage breakdown", err)
	}

	summary, err := aiUsageMetrics(trendReply.GetSummary())
	if err != nil {
		return aiusagebiz.Analysis{}, fmt.Errorf("convert AI usage summary: %w", err)
	}
	trend, err := aiUsageTrend(trendReply.GetPoints())
	if err != nil {
		return aiusagebiz.Analysis{}, err
	}
	breakdown, err := aiUsageBreakdown(breakdownReply.GetItems())
	if err != nil {
		return aiusagebiz.Analysis{}, err
	}
	return aiusagebiz.Analysis{
		Summary:   summary,
		Trend:     trend,
		Dimension: query.Dimension,
		Order:     query.Order,
		Breakdown: breakdown,
	}, nil
}

func aiUsageQueryError(operation string, err error) error {
	if isUnavailable(err) {
		return aiusagebiz.Unavailable(err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
