package traffic

import "context"

// QueryStore 查询存储层已经生成的流量聚合结果
//
// 当前生产实现使用 ClickHouse 分钟聚合表，但该细节不会进入 biz 用例
type QueryStore interface {
	QueryTrafficSummary(context.Context, Filter) (Summary, error)
	QueryTrafficTrend(context.Context, TrendQuery) ([]TrendPoint, error)
	QueryTrafficBreakdown(context.Context, BreakdownQuery) ([]BreakdownItem, error)
	QueryResourceTraffic(context.Context, ResourceTrafficQuery) ([]ResourceTrafficSummary, error)
}
