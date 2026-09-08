package biz

import (
	"context"
	"errors"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
)

// ErrQueueEmpty 表示本地队列当前没有待回放记录。
var ErrQueueEmpty = errors.New("disk queue is empty")

// QueuedBatch 表示从本地磁盘队列读取的一段连续记录。
type QueuedBatch struct {
	// Records 保持磁盘队列中的原始顺序，Kafka 写入成功前不能跳过其中任一记录。
	Records []*alsv1.RequestRecord
	// LastSequence 是本批记录成功写入 Kafka 后可以确认到的队列位置。
	LastSequence uint64
	// Bytes 是本批 protobuf 记录占用的磁盘队列数据字节数。
	Bytes int64
}

// RecordQueue 是能够顺序读取并确认的本地磁盘队列。
type RecordQueue interface {
	Write(context.Context, []*alsv1.RequestRecord) error
	// Read 读取但不删除连续队首记录。
	Read(context.Context, int) (QueuedBatch, error)
	// Commit 只确认已经完整写入 Kafka 的批次。
	Commit(context.Context, QueuedBatch) error
	// Pending 从内存快照返回尚未确认的记录数和 protobuf 字节数，不执行磁盘 I/O。
	Pending() (int64, int64)
}
