package request

import "context"

// Queries 提供请求明细查询
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
