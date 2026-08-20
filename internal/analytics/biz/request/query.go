package request

import (
	"context"
	"time"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
)

// Queries 提供不依赖 ClickHouse 协议的请求明细查询入口
type Queries struct {
	store QueryStore
}

// NewQueries 创建请求明细查询
func NewQueries(store QueryStore) *Queries {
	return &Queries{store: store}
}

// List 按时间倒序查询已经保存的请求记录
func (q *Queries) List(ctx context.Context, options ListOptions) (Page, error) {
	return q.store.ListRequests(ctx, options)
}

// Get 按稳定记录 ID 查询单次请求明细
func (q *Queries) Get(ctx context.Context, id string, startedAt time.Time) (*alsv1.RequestRecord, error) {
	return q.store.GetRequest(ctx, id, startedAt)
}
