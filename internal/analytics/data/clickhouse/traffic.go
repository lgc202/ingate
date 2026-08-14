package clickhouse

import (
	"context"

	"github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

// QueryTrafficTrend 从分钟统计查询流量和延迟趋势。
func (s *Store) QueryTrafficTrend(
	_ context.Context,
	_ traffic.TrendQuery,
) ([]traffic.TrendPoint, error) {
	return nil, errNotImplemented
}

// QueryTrafficBreakdown 从分钟统计查询资源维度分布。
func (s *Store) QueryTrafficBreakdown(
	_ context.Context,
	_ traffic.BreakdownQuery,
) ([]traffic.BreakdownItem, error) {
	return nil, errNotImplemented
}
