// Package aiusage 负责查询模型调用与 Token 用量
package aiusage

import "context"

// Query 提供不依赖 ClickHouse 协议的模型用量查询
type Query struct {
	store QueryStore
}

// NewQuery 创建模型用量查询
func NewQuery(store QueryStore) *Query {
	return &Query{store: store}
}

// Trend 查询整个时间范围的模型用量汇总及指定粒度的变化趋势
func (q *Query) Trend(ctx context.Context, query TrendQuery) (TrendResult, error) {
	summary, err := q.store.QueryAIUsageSummary(ctx, query.Filter)
	if err != nil {
		return TrendResult{}, err
	}
	points, err := q.store.QueryAIUsageTrend(ctx, query)
	if err != nil {
		return TrendResult{}, err
	}
	return TrendResult{Summary: summary, Points: points}, nil
}

// Breakdown 查询按业务维度聚合的模型调用与 Token 用量
func (q *Query) Breakdown(ctx context.Context, query BreakdownQuery) ([]BreakdownItem, error) {
	return q.store.QueryAIUsageBreakdown(ctx, query)
}
