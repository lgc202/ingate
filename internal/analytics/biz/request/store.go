package request

import "context"

// RecordStore 保存从 Kafka 接收到的请求事实
//
// Kafka Consumer 只会在 SaveRequestBatch 成功后提交 offset，因此实现必须允许同一批次重试
type RecordStore interface {
	SaveRequestBatch(context.Context, []Fact) error
}

// QueryStore 查询已经保存的请求事实
type QueryStore interface {
	ListRequests(context.Context, ListOptions) (Page, error)
}
