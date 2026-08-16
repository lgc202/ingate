// Package request 负责保存和查询单次网关请求事实
package request

import (
	"context"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
)

// Recorder 将已校验的 ALS 请求记录转换为稳定事实并持久化
type Recorder struct {
	store RecordStore
}

// NewRecorder 创建请求事实记录器
func NewRecorder(store RecordStore) *Recorder {
	return &Recorder{store: store}
}

// Record 保存一批已经通过协议边界校验的请求记录
func (r *Recorder) Record(ctx context.Context, records []*alsv1.RequestRecord) error {
	if len(records) == 0 {
		return nil
	}
	facts := make([]Fact, 0, len(records))
	for _, record := range records {
		facts = append(facts, Fact{
			Record:      record,
			StatusClass: classifyStatus(record.GetStatusCode()),
		})
	}
	return r.store.SaveRequestBatch(ctx, facts)
}

func classifyStatus(status uint32) StatusClass {
	switch {
	case status >= 500:
		return StatusClassServerError
	case status >= 400:
		return StatusClassClientError
	case status >= 100:
		return StatusClassSuccess
	default:
		return StatusClassNoResponse
	}
}
