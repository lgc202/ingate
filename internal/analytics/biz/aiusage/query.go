// Package aiusage 负责查询模型调用与 Token 用量。
package aiusage

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// QueryStore 查询模型调用产生的持久化用量聚合。
type QueryStore interface {
	QueryAIUsageSummary(context.Context, Filter) (Metrics, error)
	QueryAIUsageTrend(context.Context, TrendQuery) ([]TrendPoint, error)
	QueryAIUsageBreakdown(context.Context, BreakdownQuery) ([]BreakdownItem, error)
}

// Query 提供不依赖 ClickHouse 协议的模型用量查询。
type Query struct {
	store QueryStore
}

// NewQuery 创建模型用量查询。
func NewQuery(store QueryStore) *Query {
	return &Query{store: store}
}

// Trend 查询整个时间范围的模型用量汇总及指定粒度的变化趋势。
func (q *Query) Trend(ctx context.Context, query TrendQuery) (TrendResult, error) {
	var (
		summary Metrics
		points  []TrendPoint
	)
	group, queryCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		summary, err = q.store.QueryAIUsageSummary(queryCtx, query.Filter)
		return err
	})
	group.Go(func() error {
		var err error
		points, err = q.store.QueryAIUsageTrend(queryCtx, query)
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

// Breakdown 查询按业务维度聚合的模型调用与 Token 用量。
func (q *Query) Breakdown(ctx context.Context, query BreakdownQuery) ([]BreakdownItem, error) {
	return q.store.QueryAIUsageBreakdown(ctx, query)
}
