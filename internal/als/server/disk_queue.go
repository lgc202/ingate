package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

// DiskQueueReplayer 周期性把 Kafka 故障期间写入磁盘队列的请求记录重新投递到 Kafka。
type DiskQueueReplayer struct {
	recorder     *biz.Recorder
	logger       *slog.Logger
	interval     time.Duration
	batchSize    int
	done         chan struct{}
	running      atomic.Bool
	lifecycleMu  sync.Mutex
	cancel       context.CancelFunc
	stopping     bool
	replayFailed bool
}

// NewDiskQueueReplayer 创建磁盘队列回放任务。
func NewDiskQueueReplayer(
	config *conf.Data_DiskQueue,
	recorder *biz.Recorder,
	logger *slog.Logger,
) *DiskQueueReplayer {
	return &DiskQueueReplayer{
		recorder:  recorder,
		logger:    logger,
		interval:  config.GetReplayInterval().AsDuration(),
		batchSize: int(config.GetReplayBatchSize()),
		done:      make(chan struct{}),
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
	if runCtx.Err() != nil {
		return nil
	}
	r.replay(runCtx)
	timer := time.NewTimer(r.interval)
	defer timer.Stop()
	for {
		select {
		case <-runCtx.Done():
			return nil
		case <-timer.C:
			r.replay(runCtx)
			// 从本轮结束后重新计时，避免一次 Kafka 超时后立即消费积压的旧 tick 并连续重试
			timer.Reset(r.interval)
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
	// Kafka 写入会继承该取消信号，关闭时无需等待完整的写入超时
	cancel()
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop disk queue replayer: %w", ctx.Err())
	}
}

func (r *DiskQueueReplayer) replay(ctx context.Context) {
	for {
		replayed, err := r.recorder.ReplayBatch(ctx, r.batchSize)
		if err != nil {
			if ctx.Err() == nil && !r.replayFailed {
				// 故障状态没有变化时不按一秒重试周期重复打印相同告警
				r.logger.Warn("disk queue replay failed", "err", err)
			}
			r.replayFailed = true
			return
		}
		r.replayFailed = false
		if !replayed {
			return
		}
		// Kafka 恢复后连续排空磁盘队列，同时让优雅退出可以在批次之间及时停止
		if ctx.Err() != nil {
			return
		}
	}
}
