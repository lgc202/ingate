package server

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

// DiskQueueReplayer 周期性把 Kafka 故障期间写入磁盘队列的请求记录重新投递到 Kafka
type DiskQueueReplayer struct {
	recorder  *biz.Recorder
	logger    *slog.Logger
	interval  time.Duration
	batchSize int
	done      chan struct{}
	mu        sync.Mutex
	cancel    context.CancelFunc
	stopping  bool
}

// NewDiskQueueReplayer 创建磁盘队列回放任务
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

// Start 阻塞运行回放循环，由 Kratos App 管理其生命周期
func (r *DiskQueueReplayer) Start(ctx context.Context) error {
	replayContext, cancel := context.WithCancel(ctx)
	defer cancel()
	r.mu.Lock()
	r.cancel = cancel
	stopping := r.stopping
	r.mu.Unlock()
	if stopping {
		cancel()
	}

	defer close(r.done)
	r.replay(replayContext)
	timer := time.NewTimer(r.interval)
	defer timer.Stop()
	for {
		select {
		case <-replayContext.Done():
			return nil
		case <-timer.C:
			r.replay(replayContext)
			// 从本轮结束后重新计时，避免一次 Kafka 超时后立即消费积压的旧 tick 并连续重试
			timer.Reset(r.interval)
		}
	}
}

func (r *DiskQueueReplayer) replay(ctx context.Context) {
	for {
		replayed, err := r.recorder.ReplayBatch(ctx, r.batchSize)
		if err != nil {
			if ctx.Err() == nil {
				r.logger.Warn("disk queue replay failed", "error", err)
			}
			return
		}
		if !replayed {
			return
		}
		// Kafka 恢复后连续排空磁盘队列，同时让优雅退出可以在批次之间及时停止
		if ctx.Err() != nil {
			return
		}
	}
}

// Stop 停止回放循环并等待当前一轮处理结束
func (r *DiskQueueReplayer) Stop(ctx context.Context) error {
	r.mu.Lock()
	r.stopping = true
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		// Kafka 写入会继承该取消信号，关闭时无需等待完整的写入超时
		cancel()
	}
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
