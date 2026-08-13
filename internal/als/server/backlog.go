package server

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

// Backlog 周期性回放 Kafka 故障期间积存在本地磁盘的请求记录
type Backlog struct {
	recorder *biz.Recorder
	logger   *slog.Logger
	interval time.Duration
	batch    int
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// NewBacklog 创建积压回放任务
func NewBacklog(config *conf.Data, recorder *biz.Recorder, logger *slog.Logger) *Backlog {
	queue := config.GetDiskQueue()
	return &Backlog{
		recorder: recorder,
		logger:   logger,
		interval: queue.GetReplayInterval().AsDuration(),
		batch:    int(queue.GetReplayBatchSize()),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start 阻塞运行回放循环，由 Kratos App 管理其生命周期
func (b *Backlog) Start(ctx context.Context) error {
	defer close(b.done)
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	for {
		select {
		case <-b.stop:
			return nil
		case <-ticker.C:
			b.flush(ctx)
		}
	}
}

func (b *Backlog) flush(ctx context.Context) {
	for {
		flushed, err := b.recorder.FlushBacklog(ctx, b.batch)
		if err != nil {
			b.logger.Warn("request record backlog flush failed", "error", err)
			return
		}
		if !flushed {
			return
		}

		// Kafka 恢复后连续排空积压，而不是每个 replay_interval 只处理一批
		// 每批之间检查停止信号，使大规模积压不会拖住优雅退出
		select {
		case <-b.stop:
			return
		default:
		}
	}
}

// Stop 停止回放循环并等待当前一轮处理结束
func (b *Backlog) Stop(ctx context.Context) error {
	b.stopOnce.Do(func() { close(b.stop) })
	select {
	case <-b.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
