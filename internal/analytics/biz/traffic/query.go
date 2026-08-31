// Package traffic 负责查询网关流量和延迟聚合结果。
package traffic

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// QueryStore 查询存储层已经生成的流量聚合结果。
//
// 当前生产实现使用 ClickHouse 分钟聚合表，但该细节不会进入 biz 用例。
type QueryStore interface {
	QueryTrafficSummary(context.Context, Filter) (Summary, error)
	QueryTrafficTrend(context.Context, TrendQuery) ([]TrendPoint, error)
	QueryTrafficBreakdown(context.Context, BreakdownQuery) ([]BreakdownItem, error)
	QueryResourceTraffic(context.Context, ResourceTrafficQuery) ([]ResourceTrafficSummary, error)
}

// Query 提供不依赖 ClickHouse 协议的流量趋势和资源分布查询。
type Query struct {
	store QueryStore
}

// NewQuery 创建流量查询。
func NewQuery(store QueryStore) *Query {
	return &Query{store: store}
}

// Trend 查询整个时间范围的流量汇总及指定粒度的变化趋势。
func (q *Query) Trend(ctx context.Context, query TrendQuery) (TrendResult, error) {
	var (
		summary Summary
		points  []TrendPoint
	)
	group, queryCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		summary, err = q.store.QueryTrafficSummary(queryCtx, query.Filter)
		return err
	})
	group.Go(func() error {
		var err error
		points, err = q.store.QueryTrafficTrend(queryCtx, query)
		return err
	})
	// 汇总和趋势读取同一份分钟聚合事实，二者没有先后依赖。
	// 并行查询可避免长时间范围下的两次 ClickHouse 往返叠加，
	// 同时 errgroup 会在任一查询失败时取消另一条在途查询。
	if err := group.Wait(); err != nil {
		return TrendResult{}, err
	}
	return TrendResult{Summary: summary, Points: points}, nil
}

// Breakdown 查询按资源维度聚合的流量和延迟。
func (q *Query) Breakdown(ctx context.Context, query BreakdownQuery) ([]BreakdownItem, error) {
	return q.store.QueryTrafficBreakdown(ctx, query)
}

// ResourceTraffic 查询指定资源的列表流量摘要。
func (q *Query) ResourceTraffic(ctx context.Context, query ResourceTrafficQuery) ([]ResourceTrafficSummary, error) {
	return q.store.QueryResourceTraffic(ctx, query)
}
