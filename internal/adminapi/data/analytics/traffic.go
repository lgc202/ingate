package analytics

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
)

// TrafficRepository 通过 Analytics gRPC 查询流量聚合结果。
type TrafficRepository struct {
	client analyticsv1.TrafficServiceClient
}

// NewTrafficRepository 创建流量分析 Repository。
func NewTrafficRepository(connection *grpc.ClientConn) *TrafficRepository {
	return &TrafficRepository{client: analyticsv1.NewTrafficServiceClient(connection)}
}

// Analyze 查询同一范围内的汇总、趋势和资源排名。
func (r *TrafficRepository) Analyze(ctx context.Context, query trafficbiz.Query) (trafficbiz.Analysis, error) {
	var (
		trendReply     *analyticsv1.GetTrafficTrendResponse
		breakdownReply *analyticsv1.ListTrafficBreakdownResponse
	)
	group, queryCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		trendReply, err = r.client.GetTrafficTrend(queryCtx, &analyticsv1.GetTrafficTrendRequest{
			Filter: analyticsTrafficFilter(query.Filter),
			Bucket: analyticsTimeBucket(query.Bucket),
		})
		return trafficQueryError(ctx, "query traffic trend", err)
	})
	group.Go(func() error {
		var err error
		breakdownReply, err = r.client.ListTrafficBreakdown(queryCtx, &analyticsv1.ListTrafficBreakdownRequest{
			Filter:    analyticsTrafficFilter(query.Filter),
			Dimension: analyticsTrafficDimension(query.Dimension),
			Order:     analyticsTrafficBreakdownOrder(query.Order),
			Limit:     uint32(query.Limit),
		})
		return trafficQueryError(ctx, "query traffic breakdown", err)
	})
	// 趋势和资源排名是同一分析页面的两个独立结果。并行请求避免两次 Analytics RPC。
	// 串行占用调用方的超时预算；任一失败时取消另一条请求，不继续制造无用查询。
	if err := group.Wait(); err != nil {
		return trafficbiz.Analysis{}, err
	}
	if trendReply == nil {
		return trafficbiz.Analysis{}, errors.New("analytics returned an empty traffic trend response")
	}
	if breakdownReply == nil {
		return trafficbiz.Analysis{}, errors.New("analytics returned an empty traffic breakdown response")
	}
	if len(breakdownReply.GetItems()) > query.Limit {
		return trafficbiz.Analysis{}, errors.New("analytics returned too many traffic breakdown items")
	}

	summary, err := trafficSummary(trendReply.GetSummary())
	if err != nil {
		return trafficbiz.Analysis{}, fmt.Errorf("convert traffic summary: %w", err)
	}
	trend, err := trafficTrend(trendReply.GetPoints(), query.Filter)
	if err != nil {
		return trafficbiz.Analysis{}, err
	}
	breakdown, err := trafficBreakdown(breakdownReply.GetItems())
	if err != nil {
		return trafficbiz.Analysis{}, err
	}
	return trafficbiz.Analysis{
		Summary:   summary,
		Trend:     trend,
		Dimension: query.Dimension,
		Order:     query.Order,
		Breakdown: breakdown,
	}, nil
}

// BatchGetResourceTraffic 查询指定资源的列表流量摘要。
func (r *TrafficRepository) BatchGetResourceTraffic(
	ctx context.Context,
	query trafficbiz.ResourceTrafficQuery,
) ([]trafficbiz.ResourceTrafficSummary, error) {
	reply, err := r.client.BatchGetResourceTraffic(ctx, &analyticsv1.BatchGetResourceTrafficRequest{
		StartTime:   timestamppb.New(query.StartTime),
		EndTime:     timestamppb.New(query.EndTime),
		Dimension:   analyticsTrafficDimension(query.Dimension),
		ResourceIds: query.ResourceIDs,
	})
	if err != nil {
		return nil, trafficQueryError(ctx, "query resource traffic", err)
	}
	if reply == nil {
		return nil, errors.New("analytics returned an empty resource traffic response")
	}
	return resourceTrafficSummaries(reply.GetSummaries(), query.ResourceIDs)
}

func trafficQueryError(ctx context.Context, operation string, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if isUnavailable(ctx, err) {
		return trafficbiz.Unavailable(err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
