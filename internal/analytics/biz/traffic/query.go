// Package traffic 负责查询网关流量和延迟聚合结果
package traffic

import "context"

// Queries 提供流量趋势和资源分布查询
type Queries struct {
	store QueryStore
}

// NewQueries 创建流量查询
func NewQueries(store QueryStore) *Queries {
	return &Queries{store: store}
}

// Trend 查询指定时间粒度的流量和延迟趋势
func (q *Queries) Trend(ctx context.Context, query TrendQuery) ([]TrendPoint, error) {
	return q.store.QueryTrafficTrend(ctx, query)
}

// Breakdown 查询按资源维度聚合的流量和延迟
func (q *Queries) Breakdown(ctx context.Context, query BreakdownQuery) ([]BreakdownItem, error) {
	return q.store.QueryTrafficBreakdown(ctx, query)
}
