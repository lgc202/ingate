package biz

import (
	"context"
	"errors"
	"time"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
)

var (
	// ErrQueueEmpty 表示本地队列当前没有待回放记录。
	ErrQueueEmpty = errors.New("disk queue is empty")
	// ErrQueueFull 表示物理容量或文件系统余量不足以可靠追加一批记录。
	ErrQueueFull = errors.New("disk queue capacity is exhausted")
	// ErrQueueInvalidBatch 表示当前批次无法作为一个完整 WAL 条目保存。
	ErrQueueInvalidBatch = errors.New("disk queue batch is invalid")
)

// QueueState 表示 WAL 当前距离物理容量边界的程度。
type QueueState uint8

const (
	// QueueHealthy 表示 WAL 具有充足的容量和文件系统剩余空间。
	QueueHealthy QueueState = iota + 1
	// QueueWarning 表示 WAL 已进入需要关注的容量区间，但仍可接收记录。
	QueueWarning
	// QueueCritical 表示 WAL 接近拒绝边界，应尽快恢复 Kafka 或扩容。
	QueueCritical
	// QueueBlocked 表示 WAL 无法在容量契约内可靠追加新记录。
	QueueBlocked
)

// QueueStatus 描述 WAL 积压和物理存储容量的当前快照。
type QueueStatus struct {
	// State 是根据物理占用与文件系统剩余空间计算的容量状态。
	State QueueState
	// Writable 表示 WAL 当前能够在容量契约内追加新记录。
	Writable bool
	// PendingEntries 是尚未投递到 Kafka 的 WAL 条目数。
	PendingEntries int64
	// PendingRecords 是尚未投递到 Kafka 的记录数。
	PendingRecords int64
	// PendingBytes 是尚未投递记录的 protobuf 逻辑字节数。
	PendingBytes int64
	// OldestEnqueuedAt 是最早未确认条目的入队时间；队列为空时为零值。
	OldestEnqueuedAt time.Time
	// DiskBytes 是 WAL 目录中分段、临时文件、锁和元数据的物理字节数。
	DiskBytes int64
	// CapacityBytes 是 WAL 目录允许占用的物理字节上限。
	CapacityBytes int64
	// FreeBytes 是 WAL 所在文件系统可供当前进程使用的剩余字节数。
	FreeBytes int64
	// MinFreeBytes 是扣除分段恢复空间后仍需保留的文件系统安全余量。
	MinFreeBytes int64
}

// QueuedBatch 表示从本地磁盘队列读取的一段连续记录。
type QueuedBatch struct {
	// Records 保持磁盘队列中的原始顺序，Kafka 写入成功前不能跳过其中任一记录。
	Records []*alsv1.RequestRecord
	// LastSequence 是本批记录成功写入 Kafka 后可以确认到的队列位置。
	LastSequence uint64
	// Bytes 是本批 RequestRecord protobuf 的逻辑字节数。
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
	// Status 检查 WAL 的物理占用和文件系统剩余空间。
	Status() QueueStatus
}

// String 返回容量状态的稳定指标标签。
func (s QueueState) String() string {
	switch s {
	case QueueHealthy:
		return "healthy"
	case QueueWarning:
		return "warning"
	case QueueCritical:
		return "critical"
	case QueueBlocked:
		return "blocked"
	default:
		return "unknown"
	}
}
