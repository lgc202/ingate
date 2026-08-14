package clickhouse

import (
	"context"

	"github.com/lgc202/ingate/internal/analytics/biz/request"
)

// SaveRequestBatch 保存请求明细，ClickHouse Materialized View 随写入更新分钟统计
//
// 实现需要使用稳定记录 ID 保证 Kafka 重试不会重复计算
func (s *Store) SaveRequestBatch(_ context.Context, _ []request.Fact) error {
	return errNotImplemented
}

// ListRequests 按时间倒序查询短期保留的请求明细。
func (s *Store) ListRequests(_ context.Context, _ request.ListOptions) (request.Page, error) {
	return request.Page{}, errNotImplemented
}
