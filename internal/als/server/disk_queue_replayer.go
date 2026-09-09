package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	otelcodes "go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	alsmetrics "github.com/lgc202/ingate/internal/als/metrics"
)

// replayBackoff 记录连续回放失败后的下一档基础等待时间。
// 随机抖动只向上增加并受 max 限制，保证实际等待始终位于配置边界内。
type replayBackoff struct {
	min  time.Duration
	max  time.Duration
	next time.Duration
}

func (b *replayBackoff) reset() {
	b.next = b.min
}

func (b *replayBackoff) nextDelay() time.Duration {
	base := b.next
	if b.next >= b.max/2 {
		b.next = b.max
	} else {
		b.next *= 2
	}

	jitterLimit := min(base/2, b.max-base)
	if jitterLimit <= 0 {
		return base
	}
	return base + time.Duration(rand.Int64N(int64(jitterLimit)+1))
}

// DiskQueueReplayer 周期性把 Kafka 故障期间写入磁盘队列的请求记录重新投递到 Kafka。
// Kafka 恢复后由单个循环按队首顺序持续排空积压，避免并发回放打乱确认位置。
// 生命周期状态允许 Kratos 的 Start 和 Stop 并发到达而不遗留后台任务。
type DiskQueueReplayer struct {
	recorder    *biz.Recorder
	events      *alsmetrics.EventCollector
	logger      *slog.Logger
	tracer      oteltrace.Tracer
	batchSize   int
	backoff     replayBackoff
	done        chan struct{}
	running     atomic.Bool
	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	stopping    bool
	retryLogged bool
}

// NewDiskQueueReplayer 创建磁盘队列回放任务。
func NewDiskQueueReplayer(
	config *conf.Data_DiskQueue,
	recorder *biz.Recorder,
	events *alsmetrics.EventCollector,
	logger *slog.Logger,
	tracer oteltrace.Tracer,
) *DiskQueueReplayer {
	minBackoff := config.GetReplayMinBackoff().AsDuration()
	return &DiskQueueReplayer{
		recorder:  recorder,
		events:    events,
		logger:    logger,
		tracer:    tracer,
		batchSize: int(config.GetReplayBatchSize()),
		backoff: replayBackoff{
			min:  minBackoff,
			max:  config.GetReplayMaxBackoff().AsDuration(),
			next: minBackoff,
		},
		done: make(chan struct{}),
	}
}

// Start 阻塞运行回放循环，由 Kratos App 管理其生命周期。
func (r *DiskQueueReplayer) Start(ctx context.Context) error {
	if !r.running.CompareAndSwap(false, true) {
		return errors.New("disk queue replayer is already running")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.lifecycleMu.Lock()
	r.cancel = cancel
	stopping := r.stopping
	r.lifecycleMu.Unlock()
	if stopping {
		cancel()
	}

	defer close(r.done)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-runCtx.Done():
			return nil
		case <-timer.C:
			delay, paused := r.replay(runCtx)
			if paused {
				<-runCtx.Done()
				return nil
			}
			timer.Reset(delay)
		}
	}
}

// Stop 停止回放循环并等待当前一轮处理结束。
func (r *DiskQueueReplayer) Stop(ctx context.Context) error {
	r.lifecycleMu.Lock()
	r.stopping = true
	cancel := r.cancel
	r.lifecycleMu.Unlock()

	if cancel == nil {
		return nil
	}
	// Kafka 写入会继承该取消信号，关闭时无需等待完整的写入超时。
	cancel()

	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop disk queue replayer: %w", ctx.Err())
	}
}

// replay 连续提交可用批次，遇到空队列或失败时返回下一次调度决定。
func (r *DiskQueueReplayer) replay(ctx context.Context) (time.Duration, bool) {
	for {
		if !r.recorder.PrepareReplay(ctx) {
			r.events.SetReplayBackoff(0)
			r.backoff.reset()
			r.retryLogged = false
			return r.backoff.min, false
		}

		replayCtx, span := r.tracer.Start(
			ctx,
			"als.wal.replay",
			// 回放可能发生在原始接收 Trace 结束很久以后，应由 WAL 中保存的上下文建立 Link。
			oteltrace.WithNewRoot(),
		)
		result, err := r.recorder.ReplayBatch(replayCtx, r.batchSize)
		if err != nil {
			span.SetStatus(otelcodes.Error, "replay failed")
		}
		span.End()
		switch result {
		case biz.ReplayCommitted:
			r.events.SetReplayBackoff(0)
			r.backoff.reset()
			r.retryLogged = false
			if ctx.Err() != nil {
				return r.backoff.min, false
			}
			continue
		case biz.ReplayIdle:
			r.events.SetReplayBackoff(0)
			r.backoff.reset()
			r.retryLogged = false
			return r.backoff.min, false
		case biz.ReplayPaused:
			r.events.SetReplayBackoff(0)
			// 永久错误只会到达一次，不能被先前的临时失败日志抑制。
			if ctx.Err() == nil {
				r.logger.ErrorContext(replayCtx, "disk queue replay paused", "err", err)
			}
			return 0, true
		case biz.ReplayRetry:
			delay := r.backoff.nextDelay()
			r.events.SetReplayBackoff(delay)
			if ctx.Err() == nil && !r.retryLogged {
				r.logger.WarnContext(replayCtx, "disk queue replay failed",
					"retry_after", delay,
					"err", err,
				)
			}
			r.retryLogged = true
			return delay, false
		default:
			r.events.SetReplayBackoff(0)
			r.logger.ErrorContext(replayCtx, "disk queue replay returned invalid result", "result", result)
			return 0, true
		}
	}
}
