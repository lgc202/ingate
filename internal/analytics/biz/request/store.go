package request

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound 表示请求记录不存在或已经超过明细保留期
var ErrNotFound = errors.New("request record not found")

// RecordStore 保存从 Kafka 接收到的请求事实
type RecordStore interface {
	SaveRequestBatch(context.Context, []Record) error
}

// QueryStore 查询已经保存的请求事实
type QueryStore interface {
	ListRequests(context.Context, ListOptions) (Page, error)
	GetRequest(context.Context, string, time.Time) (*Record, error)
}
