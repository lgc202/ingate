package biz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
)

// ErrQueueEmpty 表示本地队列当前没有待回放记录。
var ErrQueueEmpty = errors.New("disk queue is empty")

// QueuedBatch 表示从本地磁盘队列读取的一段连续记录。
type QueuedBatch struct {
	// Records 保持磁盘队列中的原始顺序，Kafka 写入成功前不能跳过其中任一记录
	Records []*alsv1.RequestRecord
	// LastSequence 是本批记录成功写入 Kafka 后可以确认到的队列位置
	LastSequence uint64
	// Bytes 是本批 protobuf 记录占用的磁盘队列数据字节数
	Bytes int64
}

// RecordPublisher 是请求记录的 Kafka 发布边界。
//
// 接口定义在 biz，由 Kafka 适配器实现，避免业务层依赖具体客户端。
type RecordPublisher interface {
	Publish(context.Context, []*alsv1.RequestRecord) error
	// Ping 验证主写入端当前可以完成连接和鉴权
	Ping(context.Context) error
}

// RecordQueue 是能够顺序读取并确认的本地磁盘队列。
type RecordQueue interface {
	Write(context.Context, []*alsv1.RequestRecord) error
	// Read 读取但不删除连续队首记录
	Read(context.Context, int) (QueuedBatch, error)
	// Commit 只确认已经完整写入 Kafka 的批次
	Commit(context.Context, QueuedBatch) error
	// Pending 返回尚未确认的记录数和 protobuf 字节数
	Pending() (int64, int64)
}

// DeliveryStatus 描述 ALS 当前投递能力和磁盘积压情况。
type DeliveryStatus struct {
	// KafkaWritable 反映最近一次 Kafka 投递结果，不主动发起网络探测
	KafkaWritable bool
	// QueueWritable 反映最近一次磁盘队列读写或确认结果
	QueueWritable bool
	// Spooling 表示新记录当前直接进入磁盘队列，避免 Kafka 故障期间每批都等待网络超时
	Spooling bool
	// PendingRecords 是等待投递到 Kafka 的本地记录数
	PendingRecords int64
	// PendingBytes 是等待投递记录的 protobuf 逻辑字节数
	PendingBytes int64
}

// DeliveryCounters 是 ALS 进程启动后累计的请求记录投递计数。
type DeliveryCounters struct {
	// Accepted 是已经被 Kafka 或磁盘队列可靠接收的记录总数。
	Accepted uint64
	// Queued 是因 Kafka 不可用而进入磁盘队列的记录总数。
	Queued uint64
	// Replayed 是从磁盘队列成功重新投递并确认的记录总数。
	Replayed uint64
	// Rejected 是 Kafka 与磁盘队列都无法接收时拒绝的记录总数。
	Rejected uint64
	// Discarded 是协议边界丢弃的不完整记录和非 HTTP 记录总数。
	Discarded uint64
}

// Recorder 负责 Kafka 优先、磁盘队列兜底以及积压记录回放。
//
// 该链路采用至少一次投递：只有 Kafka 确认成功后才删除磁盘队列记录；
// 如果 Kafka 已确认而本地确认失败，
// 同一记录可能再次发布，下游必须使用稳定的 RequestRecord.id 幂等入库。
type Recorder struct {
	publisher RecordPublisher
	queue     RecordQueue
	logger    *slog.Logger
	spoolMu   sync.Mutex
	spooling  atomic.Bool
	kafkaOK   atomic.Bool
	queueOK   atomic.Bool
	accepted  atomic.Uint64
	queued    atomic.Uint64
	replayed  atomic.Uint64
	rejected  atomic.Uint64
	discarded atomic.Uint64
}

// NewRecorder 创建请求记录写入用例。
func NewRecorder(publisher RecordPublisher, queue RecordQueue, logger *slog.Logger) *Recorder {
	recorder := &Recorder{
		publisher: publisher,
		queue:     queue,
		logger:    logger,
	}
	recorder.queueOK.Store(true)
	pending, _ := queue.Pending()
	if pending > 0 {
		recorder.spooling.Store(true)
		logger.Info("pending request records found", "records", pending)
	}
	return recorder
}

// Write 接收一批已经完成的请求记录。
//
// Kafka 不可用时写入本地队列即视为接收成功，避免让 Envoy 因分析链路故障反复重连。
func (r *Recorder) Write(ctx context.Context, records []*alsv1.RequestRecord) error {
	if len(records) == 0 {
		return nil
	}
	// Envoy 断开流时请求上下文会取消，但已经收到的完整记录仍应尽力落到本地队列
	queueContext := context.WithoutCancel(ctx)
	r.spoolMu.Lock()
	if r.spooling.Load() {
		err := r.writeQueue(queueContext, records)
		r.spoolMu.Unlock()
		if err != nil {
			r.rejected.Add(uint64(len(records)))
			return err
		}
		return nil
	}
	r.spoolMu.Unlock()

	kafkaErr := r.publisher.Publish(ctx, records)
	if kafkaErr == nil {
		r.kafkaOK.Store(true)
		r.accepted.Add(uint64(len(records)))
		return nil
	}
	r.kafkaOK.Store(false)
	r.spoolMu.Lock()
	defer r.spoolMu.Unlock()
	if queueErr := r.writeQueue(queueContext, records); queueErr != nil {
		r.rejected.Add(uint64(len(records)))
		return fmt.Errorf("write request records: %w", errors.Join(kafkaErr, queueErr))
	}
	if r.spooling.CompareAndSwap(false, true) {
		r.logger.Warn("Kafka unavailable, request records switched to disk queue", "err", kafkaErr)
	}
	return nil
}

// ReplayBatch 将一批磁盘队列记录重新写入 Kafka，返回是否实际回放了一批记录。
func (r *Recorder) ReplayBatch(ctx context.Context, limit int) (bool, error) {
	for {
		batch, err := r.queue.Read(ctx, limit)
		if errors.Is(err, ErrQueueEmpty) {
			r.spoolMu.Lock()
			pendingRecords, _ := r.queue.Pending()
			if pendingRecords > 0 {
				r.spoolMu.Unlock()
				continue
			}
			r.queueOK.Store(true)
			if r.spooling.CompareAndSwap(true, false) {
				r.logger.Info("request record delivery recovered")
			}
			r.spoolMu.Unlock()
			return false, nil
		}
		if err != nil {
			r.queueOK.Store(false)
			return false, fmt.Errorf("read disk queue: %w", err)
		}
		if err := r.publisher.Publish(ctx, batch.Records); err != nil {
			r.kafkaOK.Store(false)
			r.spooling.Store(true)
			return false, fmt.Errorf("write queued records: %w", err)
		}
		r.kafkaOK.Store(true)
		if err := r.queue.Commit(ctx, batch); err != nil {
			r.queueOK.Store(false)
			return false, fmt.Errorf("commit disk queue: %w", err)
		}
		r.queueOK.Store(true)
		r.replayed.Add(uint64(len(batch.Records)))
		return true, nil
	}
}

// CheckKafka 验证请求记录的 Kafka 主投递链路是否可用。
func (r *Recorder) CheckKafka(ctx context.Context) error {
	return r.publisher.Ping(ctx)
}

// DeliveryStatus 返回无需访问外部系统即可读取的投递状态。
func (r *Recorder) DeliveryStatus() DeliveryStatus {
	records, bytes := r.queue.Pending()
	return DeliveryStatus{
		KafkaWritable:  r.kafkaOK.Load(),
		QueueWritable:  r.queueOK.Load(),
		Spooling:       r.spooling.Load(),
		PendingRecords: records,
		PendingBytes:   bytes,
	}
}

// DeliveryCounters 返回无需加锁读取的累计投递计数。
func (r *Recorder) DeliveryCounters() DeliveryCounters {
	return DeliveryCounters{
		Accepted:  r.accepted.Load(),
		Queued:    r.queued.Load(),
		Replayed:  r.replayed.Load(),
		Rejected:  r.rejected.Load(),
		Discarded: r.discarded.Load(),
	}
}

// Discard 记录在 ALS 协议边界被丢弃的不完整或非 HTTP 记录。
func (r *Recorder) Discard(count int) {
	if count > 0 {
		r.discarded.Add(uint64(count))
	}
}

func (r *Recorder) writeQueue(ctx context.Context, records []*alsv1.RequestRecord) error {
	if err := r.queue.Write(ctx, records); err != nil {
		r.queueOK.Store(false)
		return fmt.Errorf("write disk queue: %w", err)
	}
	r.queueOK.Store(true)
	r.accepted.Add(uint64(len(records)))
	r.queued.Add(uint64(len(records)))
	return nil
}
