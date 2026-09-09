package biz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
)

// RecorderStatus 描述 Recorder 当前的 Kafka、WAL 和积压状态。
type RecorderStatus struct {
	// Topic 是最近一次可确定的 Kafka Topic 契约状态。
	Topic TopicStatus
	// Queue 是 WAL 积压与物理容量状态。
	Queue QueueStatus
	// KafkaWritable 表示 Topic 合规且最近一次 Kafka 写入成功。
	KafkaWritable bool
	// Spooling 表示新记录当前直接进入磁盘队列，避免 Kafka 故障期间每批都等待网络超时。
	Spooling bool
	// ReplayPaused 表示队首记录遇到永久错误，本进程不再自动重试。
	ReplayPaused bool
}

// RecorderCounters 是 Recorder 启动后累计的请求记录处理计数。
type RecorderCounters struct {
	// Valid 是通过协议校验并进入可靠投递流程的记录总数。
	Valid uint64
	// KafkaAccepted 是 Kafka 已确认接收的记录总数，包括 WAL 重放和可能重复的发布。
	KafkaAccepted uint64
	// Spooled 是成功追加到 WAL 的记录总数。
	Spooled uint64
	// Replayed 是从 WAL 成功发布到 Kafka 的记录总数，包括本地确认失败后的重复发布。
	Replayed uint64
	// Committed 是成功发布后从 WAL 确认删除的记录总数。
	Committed uint64
	// Rejected 是 Kafka 未确认整批且磁盘队列追加失败的记录总数。
	// Kafka 可能已接收其中一部分，因此该计数不等于精确丢失量。
	Rejected uint64
	// Discarded 是协议边界丢弃的不完整记录和非 HTTP 记录总数。
	Discarded uint64
}

// ReplayResult 表示一次队首回放完成后 Replayer 应观察到的稳定结果。
type ReplayResult uint8

const (
	// ReplayIdle 表示当前没有可提交的队首条目。
	ReplayIdle ReplayResult = iota + 1
	// ReplayCommitted 表示一个连续队首批次已经发布并提交。
	ReplayCommitted
	// ReplayRetry 表示条目保持未确认，稍后可以重试。
	ReplayRetry
	// ReplayPaused 表示永久错误暂停该队首条目的自动重试。
	ReplayPaused
)

// Recorder 负责 Kafka 优先、磁盘队列兜底以及积压记录回放。
//
// 该链路采用至少一次投递：只有 Kafka 确认成功后才删除磁盘队列记录；
// Kafka 已确认而本地确认失败时，同一记录可能再次发布，
// 下游必须使用稳定的 RequestRecord.id 幂等入库。
type Recorder struct {
	publisher RecordPublisher
	topic     *TopicContract
	queue     RecordQueue
	logger    *slog.Logger
	state     *recorderState

	valid         atomic.Uint64
	kafkaAccepted atomic.Uint64
	spooled       atomic.Uint64
	replayed      atomic.Uint64
	committed     atomic.Uint64
	rejected      atomic.Uint64
	discarded     atomic.Uint64
}

// NewRecorder 创建请求记录写入用例。
// 磁盘队列已有积压时保持后续记录继续入队，避免新记录绕过尚未回放的旧记录。
func NewRecorder(
	publisher RecordPublisher,
	topic *TopicContract,
	queue RecordQueue,
	logger *slog.Logger,
) *Recorder {
	pending, _ := queue.Pending()
	return &Recorder{
		publisher: publisher,
		topic:     topic,
		queue:     queue,
		logger:    logger,
		state:     newRecorderState(pending > 0),
	}
}

// Write 接收一批已经完成的请求记录。
//
// Kafka 不可用时写入本地队列即视为接收成功，避免让 Envoy 因分析链路故障反复重连。
func (r *Recorder) Write(ctx context.Context, records []*alsv1.RequestRecord) error {
	if len(records) == 0 {
		return nil
	}
	r.valid.Add(uint64(len(records)))

	if r.state.reserveWriteTarget(r.topic.Status().Compliant) == queueTarget {
		return r.writeQueue(ctx, records)
	}

	return r.writeKafka(ctx, records)
}

// ReplayBatch 尝试发布并提交一个连续队首批次。
// 回放位置只能由单个调用方串行推进。
func (r *Recorder) ReplayBatch(ctx context.Context, limit int) (ReplayResult, error) {
	if !r.topic.Status().Compliant {
		r.state.startSpooling()
		return ReplayIdle, nil
	}

	batch, err := r.queue.Read(ctx, limit)
	if errors.Is(err, ErrQueueEmpty) {
		r.state.queueSucceeded()
		r.resumeKafkaWrites(ctx)
		return ReplayIdle, nil
	}
	if err != nil {
		r.state.queueFailed()
		return ReplayRetry, fmt.Errorf("read disk queue: %w", err)
	}

	result := r.publisher.Publish(ctx, batch.Records)
	if result.Err != nil {
		r.kafkaAccepted.Add(uint64(result.Confirmed))
		r.replayed.Add(uint64(result.Confirmed))
		r.state.replayFailed(result.Class)
		if result.Class == PublishPermanent {
			return ReplayPaused, fmt.Errorf("write queued records: %w", result.Err)
		}
		return ReplayRetry, fmt.Errorf("write queued records: %w", result.Err)
	}

	r.kafkaAccepted.Add(uint64(len(batch.Records)))
	r.replayed.Add(uint64(len(batch.Records)))
	r.state.replaySucceeded()
	if err := r.queue.Commit(ctx, batch); err != nil {
		r.state.queueFailed()
		return ReplayRetry, fmt.Errorf("commit disk queue: %w", err)
	}

	r.state.queueSucceeded()
	r.committed.Add(uint64(len(batch.Records)))
	// 最后一批应在当前回放 Trace 内完成状态恢复，便于恢复日志与故障链路关联。
	r.resumeKafkaWrites(ctx)

	return ReplayCommitted, nil
}

// PrepareReplay 在创建回放 Trace 前确认 Kafka 合规且队列已有积压。
// 队列为空时同步完成直写恢复；并发追加会通过在途计数阻止错误恢复，
// 并留待下一轮回放。
func (r *Recorder) PrepareReplay(ctx context.Context) bool {
	if !r.topic.Status().Compliant {
		r.state.startSpooling()
		return false
	}
	pending, _ := r.queue.Pending()
	if pending > 0 {
		return true
	}

	r.state.queueSucceeded()
	r.resumeKafkaWrites(ctx)
	return false
}

// Status 返回无需访问外部系统即可读取的 Recorder 状态。
func (r *Recorder) Status() RecorderStatus {
	return r.state.status(r.topic.Status(), r.queue.Status())
}

// Counters 返回无需加锁读取的累计处理计数。
func (r *Recorder) Counters() RecorderCounters {
	return RecorderCounters{
		Valid:         r.valid.Load(),
		KafkaAccepted: r.kafkaAccepted.Load(),
		Spooled:       r.spooled.Load(),
		Replayed:      r.replayed.Load(),
		Committed:     r.committed.Load(),
		Rejected:      r.rejected.Load(),
		Discarded:     r.discarded.Load(),
	}
}

// Discard 记录在 ALS 协议边界被丢弃的不完整或非 HTTP 记录。
func (r *Recorder) Discard(count int) {
	if count > 0 {
		r.discarded.Add(uint64(count))
	}
}

func (r *Recorder) resumeKafkaWrites(ctx context.Context) {
	if r.state.resumeKafkaWrites(r.topic.Status().Compliant, r.queueEmpty) {
		r.logger.InfoContext(ctx, "request record publishing recovered")
	}
}

func (r *Recorder) writeKafka(ctx context.Context, records []*alsv1.RequestRecord) error {
	result := r.publisher.Publish(ctx, records)
	if result.Err == nil {
		r.state.completeKafkaWrite()
		r.kafkaAccepted.Add(uint64(len(records)))
		return nil
	}
	r.kafkaAccepted.Add(uint64(result.Confirmed))

	if r.state.failKafkaWrite() {
		r.logger.WarnContext(ctx, "Kafka write failed; request records switched to disk queue",
			"confirmed", result.Confirmed,
			"failed", result.Failed,
			"class", result.Class.String(),
			"err", result.Err,
		)
	}

	if err := r.writeQueue(ctx, records); err != nil {
		return fmt.Errorf("write request records: %w", errors.Join(result.Err, err))
	}

	return nil
}

// writeQueue 完成 reserveWriteTarget 或 failKafkaWrite 登记的磁盘队列写入。
func (r *Recorder) writeQueue(ctx context.Context, records []*alsv1.RequestRecord) error {
	// 流取消不应丢弃已经完整接收的记录，但仍保留 Trace 和日志所需的上下文值。
	err := r.queue.Write(context.WithoutCancel(ctx), records)
	changed := r.state.finishQueueWrite(err)

	if err != nil {
		r.rejected.Add(uint64(len(records)))
		if changed {
			r.logger.ErrorContext(ctx, "disk queue write failed", "err", err)
		}
		return fmt.Errorf("write disk queue: %w", err)
	}

	if changed {
		r.logger.InfoContext(ctx, "disk queue recovered")
	}

	r.spooled.Add(uint64(len(records)))

	return nil
}

func (r *Recorder) queueEmpty() bool {
	pending, _ := r.queue.Pending()
	return pending == 0
}
