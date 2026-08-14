package request

import "context"

// RecordStore 保存从 Kafka 接收到的请求事实
//
// 实现必须使用 idempotencyKey 吸收完全相同的批次重试
type RecordStore interface {
	SaveRequestBatch(ctx context.Context, idempotencyKey string, facts []Fact) error
}

// QueryStore 查询已经保存的请求事实
type QueryStore interface {
	ListRequests(context.Context, ListOptions) (Page, error)
}
