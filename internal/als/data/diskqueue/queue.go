// Package diskqueue 使用 tidwall/wal 实现请求记录的本地磁盘队列。
package diskqueue

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/tidwall/wal"
	"google.golang.org/protobuf/proto"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

// ErrFull 表示本地队列已达到配置的逻辑容量上限。
var ErrFull = errors.New("disk queue is full")

// Queue 保存 Kafka 暂时不可用期间尚未投递的请求记录。
type Queue struct {
	log      *wal.Log
	maxBytes int64
	mu       sync.Mutex
	records  atomic.Int64
	bytes    atomic.Int64
}

// NewQueue 打开本地磁盘队列，允许已确认记录全部清空。
//
// 启动时扫描未确认记录恢复计数；队列损坏会直接阻止服务启动，
// 避免悄悄跳过尚未投递的数据。
func NewQueue(config *conf.Data_DiskQueue) (*Queue, error) {
	log, err := wal.Open(config.GetPath(), &wal.Options{
		NoSync:           !config.GetSync(),
		SegmentSize:      int(config.GetSegmentBytes()),
		LogFormat:        wal.Binary,
		SegmentCacheSize: 2,
		AllowEmpty:       true,
	})
	if err != nil {
		return nil, fmt.Errorf("open disk queue: %w", err)
	}
	records, bytes, err := queueUsage(log)
	if err != nil {
		_ = log.Close()
		return nil, err
	}
	queue := &Queue{log: log, maxBytes: config.GetMaxBytes()}
	queue.records.Store(records)
	queue.bytes.Store(bytes)
	return queue, nil
}

// Write 以连续序号把一批请求记录原子追加到磁盘。
//
// max_bytes 约束的是尚未确认记录的 protobuf 字节数，
// 队列索引和预分配空间不计入该逻辑配额。
func (q *Queue) Write(ctx context.Context, records []*alsv1.RequestRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	last, err := q.log.LastIndex()
	if err != nil {
		return fmt.Errorf("read last queue index: %w", err)
	}
	if uint64(len(records)) > math.MaxUint64-last {
		return errors.New("disk queue sequence is exhausted")
	}
	usedBytes := q.bytes.Load()
	if usedBytes > q.maxBytes {
		return ErrFull
	}
	availableBytes := q.maxBytes - usedBytes
	batch := new(wal.Batch)
	var batchBytes int64
	for i, record := range records {
		value, err := proto.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal request record: %w", err)
		}
		recordBytes := int64(len(value))
		if recordBytes > availableBytes-batchBytes {
			return ErrFull
		}
		batchBytes += recordBytes
		batch.Write(last+uint64(i)+1, value)
	}
	if err := q.log.WriteBatch(batch); err != nil {
		return fmt.Errorf("append disk queue: %w", err)
	}
	q.records.Add(int64(len(records)))
	q.bytes.Add(batchBytes)
	return nil
}

// Read 从队首读取最多 limit 条记录，只有 Commit 后记录才会移除。
//
// Read 和 Commit 分离使 Kafka 写入失败时记录仍留在磁盘队列；
// Kafka 已成功而 Commit 失败时可能重复投递，
// 下游应使用 RequestRecord.id 幂等入库，这是该链路明确选择的至少一次语义。
func (q *Queue) Read(ctx context.Context, limit int) (biz.QueuedBatch, error) {
	if err := ctx.Err(); err != nil {
		return biz.QueuedBatch{}, err
	}
	if limit <= 0 {
		return biz.QueuedBatch{}, errors.New("disk queue read limit must be greater than zero")
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	first, err := q.log.FirstIndex()
	if err != nil {
		return biz.QueuedBatch{}, fmt.Errorf("read first queue index: %w", err)
	}
	last, err := q.log.LastIndex()
	if err != nil {
		return biz.QueuedBatch{}, fmt.Errorf("read last queue index: %w", err)
	}
	if first > last {
		return biz.QueuedBatch{}, biz.ErrQueueEmpty
	}

	count := min(uint64(limit), last-first+1)
	end := first + count - 1
	records := make([]*alsv1.RequestRecord, 0, end-first+1)
	var bytes int64
	for sequence := first; ; sequence++ {
		value, err := q.log.Read(sequence)
		if err != nil {
			return biz.QueuedBatch{}, fmt.Errorf("read disk queue sequence %d: %w", sequence, err)
		}
		record := new(alsv1.RequestRecord)
		if err := proto.Unmarshal(value, record); err != nil {
			return biz.QueuedBatch{}, fmt.Errorf("unmarshal disk queue sequence %d: %w", sequence, err)
		}
		bytes += int64(len(value))
		records = append(records, record)
		if sequence == end {
			break
		}
	}
	return biz.QueuedBatch{Records: records, LastSequence: end, Bytes: bytes}, nil
}

// Commit 删除已经成功写入 Kafka 的连续队首记录。
func (q *Queue) Commit(ctx context.Context, batch biz.QueuedBatch) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	first, err := q.log.FirstIndex()
	if err != nil {
		return fmt.Errorf("read first queue index: %w", err)
	}
	last, err := q.log.LastIndex()
	if err != nil {
		return fmt.Errorf("read last queue index: %w", err)
	}
	if batch.LastSequence < first || batch.LastSequence > last {
		return fmt.Errorf("commit disk queue sequence %d outside [%d, %d]", batch.LastSequence, first, last)
	}
	if err := q.log.TruncateFront(batch.LastSequence + 1); err != nil {
		return fmt.Errorf("truncate disk queue: %w", err)
	}
	q.records.Add(-int64(batch.LastSequence - first + 1))
	q.bytes.Add(-batch.Bytes)
	return nil
}

// Pending 返回当前待回放的记录数和 protobuf 数据字节数。
func (q *Queue) Pending() (int64, int64) {
	return q.records.Load(), q.bytes.Load()
}

// Close 将磁盘队列缓冲同步并关闭文件。
func (q *Queue) Close() error {
	return q.log.Close()
}

func queueUsage(log *wal.Log) (int64, int64, error) {
	first, err := log.FirstIndex()
	if err != nil {
		return 0, 0, fmt.Errorf("read first queue index: %w", err)
	}
	last, err := log.LastIndex()
	if err != nil {
		return 0, 0, fmt.Errorf("read last queue index: %w", err)
	}
	var records int64
	var bytes int64
	for index := first; index <= last; index++ {
		value, err := log.Read(index)
		if err != nil {
			return 0, 0, fmt.Errorf("read disk queue sequence %d: %w", index, err)
		}
		records++
		bytes += int64(len(value))
		if index == last {
			break
		}
	}
	return records, bytes, nil
}
