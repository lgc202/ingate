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
	recorder *biz.Recorder
	logger   *slog.Logger
	interval time.Duration
	batch    int
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// NewDiskQueueReplayer 创建磁盘队列回放任务
func NewDiskQueueReplayer(
	config *conf.Data_DiskQueue,
	recorder *biz.Recorder,
	logger *slog.Logger,
) *DiskQueueReplayer {
	return &DiskQueueReplayer{
		recorder: recorder,
		logger:   logger,
		interval: config.GetReplayInterval().AsDuration(),
		batch:    int(config.GetReplayBatchSize()),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start 阻塞运行回放循环，由 Kratos App 管理其生命周期
func (r *DiskQueueReplayer) Start(ctx context.Context) error {
	defer close(r.done)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return nil
		case <-ticker.C:
			r.replay(ctx)
		}
	}
}

func (r *DiskQueueReplayer) replay(ctx context.Context) {
	for {
		replayed, err := r.recorder.ReplayBatch(ctx, r.batch)
		if err != nil {
			r.logger.Warn("disk queue replay failed", "error", err)
			return
		}
		if !replayed {
			return
		}

		// Kafka 恢复后连续排空磁盘队列，每批之间仍检查停止信号，避免大量积压拖住优雅退出
		select {
		case <-r.stop:
			return
		default:
		}
	}
}

// Stop 停止回放循环并等待当前一轮处理结束
func (r *DiskQueueReplayer) Stop(ctx context.Context) error {
	r.stopOnce.Do(func() { close(r.stop) })
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
