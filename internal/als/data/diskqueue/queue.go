// Package diskqueue 使用 tidwall/wal 保存版本化、可校验的请求记录批次。
package diskqueue

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tidwall/wal"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

// pendingUsage 是发布给健康检查和指标读取的不可变队列用量。
// 每次写入或确认都创建新值，避免读取方观察到只更新了一半的计数。
type pendingUsage struct {
	records int64
	bytes   int64
}

// Queue 保存 Kafka 暂时不可用期间尚未投递的请求记录。
// Write、Read 和 Commit 由同一把锁串行化以保持严格的队首顺序，
// Pending 从原子快照读取，供 Recorder 的并发状态迁移使用；
// Status 扫描真实磁盘占用，供就绪检查和指标采集使用。
type Queue struct {
	log     *wal.Log
	lock    *directoryLock
	path    string
	policy  capacityPolicy
	probe   storageProbe
	storage storageUsage
	mu      sync.Mutex
	pending atomic.Pointer[pendingUsage]
}

// NewQueue 排他打开本地磁盘队列，允许已确认记录全部清空。
//
// 启动时扫描未确认记录恢复计数；队列损坏会直接阻止服务启动，
// 避免悄悄跳过尚未投递的数据。
func NewQueue(config *conf.Data_DiskQueue) (*Queue, error) {
	return openQueueWithProbe(config, inspectStorage)
}

// Write 将一个请求记录批次编码为单个 WAL 条目并原子追加到磁盘。
//
// 追加前同时检查 WAL 目录的物理占用和文件系统剩余空间；
// 拒绝不会改动既有条目，也不会把记录转存到内存。
func (q *Queue) Write(ctx context.Context, records []*alsv1.RequestRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("%w: batch must contain at least one request record", biz.ErrQueueInvalidBatch)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	value, batchBytes, err := encodeEntry(ctx, records, time.Now())
	if err != nil {
		return fmt.Errorf("%w: %v", biz.ErrQueueInvalidBatch, err)
	}

	last, err := q.log.LastIndex()
	if err != nil {
		return fmt.Errorf("read last queue index: %w", err)
	}
	// 最大序列号留作 TruncateFront 清空队列时的右边界，避免确认位置加一溢出。
	if last == math.MaxUint64 {
		return errors.New("disk queue sequence is exhausted")
	}

	pending := q.pending.Load()
	storage, err := q.probe(q.path)
	if err != nil {
		return err
	}
	q.storage = storage
	encodedGrowth := int64(len(value)) + binary.MaxVarintLen64
	if encodedGrowth > q.policy.segmentBytes {
		return fmt.Errorf("%w: encoded entry exceeds the configured segment size", biz.ErrQueueInvalidBatch)
	}
	growth, ok := allocationSize(encodedGrowth, storage.blockBytes)
	if !ok {
		return biz.ErrQueueFull
	}
	if !q.policy.admits(storage, growth) {
		return biz.ErrQueueFull
	}

	if err := q.log.Write(last+1, value); err != nil {
		return fmt.Errorf("append disk queue: %w", err)
	}
	nextPending := pendingUsage{
		records: pending.records + int64(len(records)),
		bytes:   pending.bytes + batchBytes,
	}
	q.pending.Store(&nextPending)

	return nil
}

// Read 从队首读取最多 limit 条记录，只有 Commit 后记录才会移除。
// 单个 WAL 条目超过 limit 时仍会完整返回，批次不会因回放限制而被拆开或永久阻塞。
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

	records := make([]*alsv1.RequestRecord, 0, limit)
	lastSequence := first
	var bytes int64
	for sequence := first; ; sequence++ {
		value, err := q.log.Read(sequence)
		if err != nil {
			return biz.QueuedBatch{}, fmt.Errorf("read disk queue sequence %d: %w", sequence, err)
		}
		entryRecords, entryBytes, err := decodeEntry(value)
		if err != nil {
			return biz.QueuedBatch{}, fmt.Errorf("decode disk queue sequence %d: %w", sequence, err)
		}
		if len(records) > 0 && len(records)+len(entryRecords) > limit {
			break
		}

		records = append(records, entryRecords...)
		bytes += entryBytes
		lastSequence = sequence
		if len(records) >= limit || sequence == last {
			break
		}
	}

	return biz.QueuedBatch{
		Records:      records,
		LastSequence: lastSequence,
		Bytes:        bytes,
	}, nil
}

// Commit 删除已经成功写入 Kafka 的连续队首记录。
// 截断 WAL 前重新核对序号、记录数和字节数，避免错误批次确认其他尚未投递的数据。
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
	if batch.LastSequence == math.MaxUint64 {
		return errors.New("disk queue sequence is exhausted")
	}

	committed, err := measureEntries(q.log, first, batch.LastSequence)
	if err != nil {
		return err
	}
	if committed.records != int64(len(batch.Records)) || committed.bytes != batch.Bytes {
		return errors.New("commit disk queue batch metadata is inconsistent")
	}

	pending := q.pending.Load()
	if committed.records == 0 || committed.records > pending.records || committed.bytes > pending.bytes {
		return errors.New("commit disk queue batch metadata is inconsistent")
	}
	if err := q.log.TruncateFront(batch.LastSequence + 1); err != nil {
		return fmt.Errorf("truncate disk queue: %w", err)
	}

	nextPending := pendingUsage{
		records: pending.records - committed.records,
		bytes:   pending.bytes - committed.bytes,
	}
	q.pending.Store(&nextPending)

	return nil
}

// Pending 返回当前待回放的记录数和 protobuf 数据字节数。
func (q *Queue) Pending() (int64, int64) {
	usage := q.pending.Load()
	return usage.records, usage.bytes
}

// Status 返回包含实时物理占用和文件系统剩余空间的 WAL 状态。
func (q *Queue) Status() biz.QueueStatus {
	q.mu.Lock()
	defer q.mu.Unlock()

	pending := *q.pending.Load()
	storage, err := q.probe(q.path)
	if err != nil {
		status := q.policy.status(pending, q.storage)
		status.State = biz.QueueBlocked
		status.Writable = false
		return status
	}
	q.storage = storage
	return q.policy.status(pending, storage)
}

// Close 将磁盘队列缓冲同步并关闭文件，同时释放目录排他锁。
func (q *Queue) Close() error {
	if err := errors.Join(q.log.Close(), q.lock.Close()); err != nil {
		return fmt.Errorf("close disk queue: %w", err)
	}
	return nil
}

func openQueueWithProbe(config *conf.Data_DiskQueue, probe storageProbe) (*Queue, error) {
	path, err := prepareDirectory(config.GetPath())
	if err != nil {
		return nil, err
	}
	lock, err := lockDirectory(path)
	if err != nil {
		return nil, err
	}
	if err := restrictExistingFiles(path); err != nil {
		return nil, errors.Join(err, lock.Close())
	}

	// tidwall/wal 在 NoSync=false 时只有追加和文件同步都成功才从 Write 返回。
	queueLog, err := wal.Open(path, &wal.Options{
		NoSync:           !config.GetSync(),
		SegmentSize:      int(config.GetSegmentBytes()),
		LogFormat:        wal.Binary,
		SegmentCacheSize: 2,
		AllowEmpty:       true,
		DirPerms:         directoryMode,
		FilePerms:        fileMode,
	})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open disk queue: %w", err), lock.Close())
	}
	usage, err := scanPendingUsage(queueLog)
	if err != nil {
		return nil, errors.Join(err, queueLog.Close(), lock.Close())
	}
	policy := capacityPolicy{
		capacityBytes: config.GetCapacityBytes(),
		minFreeBytes:  config.GetMinFreeBytes(),
		segmentBytes:  config.GetSegmentBytes(),
	}
	storage, err := probe(path)
	if err != nil {
		return nil, errors.Join(err, queueLog.Close(), lock.Close())
	}
	queue := &Queue{
		log:     queueLog,
		lock:    lock,
		path:    path,
		policy:  policy,
		probe:   probe,
		storage: storage,
	}
	queue.pending.Store(&usage)
	return queue, nil
}

func scanPendingUsage(queueLog *wal.Log) (pendingUsage, error) {
	first, err := queueLog.FirstIndex()
	if err != nil {
		return pendingUsage{}, fmt.Errorf("read first queue index: %w", err)
	}
	last, err := queueLog.LastIndex()
	if err != nil {
		return pendingUsage{}, fmt.Errorf("read last queue index: %w", err)
	}
	return measureEntries(queueLog, first, last)
}

func measureEntries(queueLog *wal.Log, first, last uint64) (pendingUsage, error) {
	var usage pendingUsage
	for sequence := first; sequence <= last; sequence++ {
		value, err := queueLog.Read(sequence)
		if err != nil {
			return pendingUsage{}, fmt.Errorf("read disk queue sequence %d: %w", sequence, err)
		}
		records, bytes, err := decodeEntry(value)
		if err != nil {
			return pendingUsage{}, fmt.Errorf("decode disk queue sequence %d: %w", sequence, err)
		}
		if int64(len(records)) > math.MaxInt64-usage.records || bytes > math.MaxInt64-usage.bytes {
			return pendingUsage{}, errors.New("disk queue usage exceeds the supported range")
		}
		usage.records += int64(len(records))
		usage.bytes += bytes
		if sequence == last {
			break
		}
	}

	return usage, nil
}
