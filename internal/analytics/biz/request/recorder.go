// Package request 负责保存和查询单次网关请求事实。
package request

import (
	"context"
)

// RecordStore 保存从 Kafka 接收到的请求事实。
type RecordStore interface {
	SaveRequestBatch(context.Context, []Record) error
}

// Recorder 为已校验的请求记录补充结果分类并持久化。
type Recorder struct {
	store RecordStore
}

// NewRecorder 创建请求事实记录器。
func NewRecorder(store RecordStore) *Recorder {
	return &Recorder{store: store}
}

// Save 保存一批已经通过 Kafka 协议边界校验的请求记录。
func (r *Recorder) Save(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	for i := range records {
		records[i].StatusClass = ClassifyStatusCode(records[i].StatusCode)
	}
	return r.store.SaveRequestBatch(ctx, records)
}
