package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	conversationbiz "github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/conf"
)

const workerErrorDelay = 5 * time.Second

// RunWorker 从 MySQL 领取排队中的 Run，并维持模型执行期间的实例租约。
//
// 一个进程包含固定数量的执行槽，不同会话可以并发执行。部署多个进程时，MySQL 的
// SKIP LOCKED 负责分配任务，worker_id 和租约阻止失联或过期实例提交结果。
type RunWorker struct {
	conversations *conversationbiz.Service
	logger        *slog.Logger
	concurrency   int
	pollInterval  time.Duration
	leaseDuration time.Duration
	instanceID    string
	done          chan struct{}
	lifecycleMu   sync.Mutex
	cancel        context.CancelFunc
	stopping      bool
}

// NewRunWorker 创建由 Kratos 管理生命周期的 Assistant 后台 Worker。
func NewRunWorker(
	config *conf.Worker,
	conversations *conversationbiz.Service,
	logger *slog.Logger,
) *RunWorker {
	return &RunWorker{
		conversations: conversations,
		logger:        logger,
		concurrency:   int(config.GetConcurrency()),
		pollInterval:  config.GetPollInterval().AsDuration(),
		leaseDuration: config.GetLeaseDuration().AsDuration(),
		instanceID:    uuid.NewString(),
		done:          make(chan struct{}),
	}
}

// Start 启动固定数量的执行槽和一个租约恢复循环，并等待进程停止。
func (w *RunWorker) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	w.lifecycleMu.Lock()
	w.cancel = cancel
	stopping := w.stopping
	w.lifecycleMu.Unlock()
	if stopping {
		cancel()
	}
	defer close(w.done)

	var group sync.WaitGroup
	group.Add(w.concurrency + 1)
	go func() {
		defer group.Done()
		w.recoverExpiredRuns(ctx)
	}()
	for slot := 1; slot <= w.concurrency; slot++ {
		workerID := fmt.Sprintf("%s/%d", w.instanceID, slot)
		go func() {
			defer group.Done()
			w.executeRuns(ctx, workerID, slot)
		}()
	}
	group.Wait()
	return nil
}

// executeRuns 串行使用一个执行槽；多个槽之间通过数据库原子领取实现并发。
func (w *RunWorker) executeRuns(ctx context.Context, workerID string, slot int) {
	for {
		if ctx.Err() != nil {
			return
		}

		claimed, err := w.conversations.ExecuteNext(ctx, workerID, w.leaseDuration)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			w.logger.Error("assistant run execution failed", "slot", slot, "err", err)
			if !w.wait(ctx, workerErrorDelay) {
				return
			}
			continue
		}
		if claimed {
			continue
		}
		if !w.wait(ctx, w.pollInterval) {
			return
		}
	}
}

// recoverExpiredRuns 独立回收已经失去租约持有者的 Run，避免每个执行槽重复扫描。
func (w *RunWorker) recoverExpiredRuns(ctx context.Context) {
	for {
		count, err := w.conversations.RecoverExpiredRuns(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			w.logger.Error("recover expired assistant runs failed", "err", err)
			if !w.wait(ctx, workerErrorDelay) {
				return
			}
			continue
		}
		if count > 0 {
			w.logger.Warn("expired assistant runs marked as failed", "count", count)
		}
		if !w.wait(ctx, w.leaseDuration) {
			return
		}
	}
}

// Stop 取消正在执行的模型调用，并等待 Run 写入失败或取消终态。
func (w *RunWorker) Stop(ctx context.Context) error {
	w.lifecycleMu.Lock()
	w.stopping = true
	cancel := w.cancel
	w.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop assistant run worker: %w", ctx.Err())
	}
}

func (w *RunWorker) wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
