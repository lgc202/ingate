package analytics

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	trafficbiz "github.com/lgc202/ingate/internal/adminapi/biz/traffic"
)

// TrafficRepository 通过 Analytics gRPC 查询流量聚合结果
type TrafficRepository struct {
	client analyticsv1.TrafficServiceClient
}

// NewTrafficRepository 创建流量分析 Repository
func NewTrafficRepository(connection *grpc.ClientConn) *TrafficRepository {
	return &TrafficRepository{client: analyticsv1.NewTrafficServiceClient(connection)}
}

// Analyze 查询同一范围内的汇总、趋势和资源排名
func (r *TrafficRepository) Analyze(ctx context.Context, query trafficbiz.Query) (trafficbiz.Analysis, error) {
	filter := &analyticsv1.TrafficFilter{
		StartTime:  timestamppb.New(query.Filter.StartTime),
		EndTime:    timestamppb.New(query.Filter.EndTime),
		GatewayId:  query.Filter.GatewayID,
		RouteId:    query.Filter.RouteID,
		UpstreamId: query.Filter.ServiceID,
	}
	trendReply, err := r.client.GetTrafficTrend(ctx, &analyticsv1.GetTrafficTrendRequest{
		Filter: filter,
		Bucket: analyticsTimeBucket(query.Bucket),
	})
	if err != nil {
		return trafficbiz.Analysis{}, trafficQueryError("query traffic trend", err)
	}
	breakdownReply, err := r.client.ListTrafficBreakdown(ctx, &analyticsv1.ListTrafficBreakdownRequest{
		Filter:    filter,
		Dimension: analyticsTrafficDimension(query.Dimension),
		Order:     analyticsTrafficBreakdownOrder(query.Order),
		Limit:     uint32(query.Limit),
	})
	if err != nil {
		return trafficbiz.Analysis{}, trafficQueryError("query traffic breakdown", err)
	}

	summary, err := trafficSummary(trendReply.GetSummary())
	if err != nil {
		return trafficbiz.Analysis{}, fmt.Errorf("convert traffic summary: %w", err)
	}
	trend, err := trafficTrend(trendReply.GetPoints())
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

// BatchGetResourceTraffic 查询指定资源的列表流量摘要
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
		return nil, trafficQueryError("query resource traffic", err)
	}
	return resourceTrafficSummaries(reply.GetSummaries())
}

func trafficQueryError(operation string, err error) error {
	if isUnavailable(err) {
		return trafficbiz.Unavailable(err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
