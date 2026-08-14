package traffic

import "context"

// QueryStore 查询 ClickHouse 已生成的流量聚合结果
type QueryStore interface {
	QueryTrafficTrend(context.Context, TrendQuery) ([]TrendPoint, error)
	QueryTrafficBreakdown(context.Context, BreakdownQuery) ([]BreakdownItem, error)
}
