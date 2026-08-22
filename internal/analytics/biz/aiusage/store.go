package aiusage

import "context"

// QueryStore 查询模型调用产生的持久化用量聚合
type QueryStore interface {
	QueryAIUsageSummary(context.Context, Filter) (Metrics, error)
	QueryAIUsageTrend(context.Context, TrendQuery) ([]TrendPoint, error)
	QueryAIUsageBreakdown(context.Context, BreakdownQuery) ([]BreakdownItem, error)
}
