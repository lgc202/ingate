package request

import (
	"context"
	"time"
)

// Query 提供不依赖 ClickHouse 协议的请求明细查询入口
type Query struct {
	store QueryStore
}

// NewQuery 创建请求明细查询
func NewQuery(store QueryStore) *Query {
	return &Query{store: store}
}

// List 按时间倒序查询已经保存的请求记录
func (q *Query) List(ctx context.Context, options ListOptions) (Page, error) {
	return q.store.ListRequests(ctx, options)
}

// Get 按稳定记录 ID 查询单次请求明细
func (q *Query) Get(ctx context.Context, id string, startedAt time.Time) (*Record, error) {
	return q.store.GetRequest(ctx, id, startedAt)
}
