package analytics

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	aiusagebiz "github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
)

// AIUsageRepository 通过 Analytics gRPC 查询模型调用与 Token 聚合结果。
type AIUsageRepository struct {
	client analyticsv1.AIUsageServiceClient
}

// NewAIUsageRepository 创建模型用量分析 Repository。
func NewAIUsageRepository(connection *grpc.ClientConn) *AIUsageRepository {
	return &AIUsageRepository{client: analyticsv1.NewAIUsageServiceClient(connection)}
}

// Analyze 查询同一范围内的模型用量汇总、趋势与排名。
func (r *AIUsageRepository) Analyze(ctx context.Context, query aiusagebiz.Query) (aiusagebiz.Analysis, error) {
	var (
		trendReply     *analyticsv1.GetAIUsageTrendResponse
		breakdownReply *analyticsv1.ListAIUsageBreakdownResponse
	)
	group, queryCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		trendReply, err = r.client.GetAIUsageTrend(queryCtx, &analyticsv1.GetAIUsageTrendRequest{
			Filter: analyticsAIUsageFilter(query.Filter),
			Bucket: analyticsAIUsageTimeBucket(query.Bucket),
		})
		return aiUsageQueryError(ctx, "query AI usage trend", err)
	})
	group.Go(func() error {
		var err error
		breakdownReply, err = r.client.ListAIUsageBreakdown(
			queryCtx,
			&analyticsv1.ListAIUsageBreakdownRequest{
				Filter:    analyticsAIUsageFilter(query.Filter),
				Dimension: analyticsAIUsageDimension(query.Dimension),
				Order:     analyticsAIUsageBreakdownOrder(query.Order),
				Limit:     uint32(query.Limit),
			},
		)
		return aiUsageQueryError(ctx, "query AI usage breakdown", err)
	})
	// 趋势和排名是同一分析页的独立结果；并行请求共用超时预算，
	// 任一失败时取消另一条请求。
	if err := group.Wait(); err != nil {
		return aiusagebiz.Analysis{}, err
	}
	if trendReply == nil {
		return aiusagebiz.Analysis{}, errors.New("analytics returned an empty AI usage trend response")
	}
	if breakdownReply == nil {
		return aiusagebiz.Analysis{}, errors.New("analytics returned an empty AI usage breakdown response")
	}
	if len(breakdownReply.GetItems()) > query.Limit {
		return aiusagebiz.Analysis{}, errors.New("analytics returned too many AI usage breakdown items")
	}

	summary, err := aiUsageMetrics(trendReply.GetSummary())
	if err != nil {
		return aiusagebiz.Analysis{}, fmt.Errorf("convert AI usage summary: %w", err)
	}
	trend, err := aiUsageTrend(trendReply.GetPoints(), query.Filter)
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

func aiUsageQueryError(ctx context.Context, operation string, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if isUnavailable(ctx, err) {
		return aiusagebiz.Unavailable(err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
