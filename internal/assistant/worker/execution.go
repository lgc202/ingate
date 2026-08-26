// Package worker 承载由 Kratos 管理生命周期的后台任务。
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/conf"
)

const executionErrorDelay = 5 * time.Second

// ExecutionWorker 从 MySQL 领取排队任务，并维持模型执行期间的实例租约。
//
// 一个进程包含固定数量的执行槽，不同会话可以并发执行。部署多个进程时，MySQL 的
// SKIP LOCKED 负责分配任务，workerID 和租约阻止失联或过期实例提交结果。
type ExecutionWorker struct {
	executor      *execution.Executor
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

// NewExecutionWorker 创建由 Kratos 管理生命周期的后台执行服务。
func NewExecutionWorker(
	config *conf.Worker,
	executor *execution.Executor,
	logger *slog.Logger,
) *ExecutionWorker {
	return &ExecutionWorker{
		executor:      executor,
		logger:        logger,
		concurrency:   int(config.GetConcurrency()),
		pollInterval:  config.GetPollInterval().AsDuration(),
		leaseDuration: config.GetLeaseDuration().AsDuration(),
		instanceID:    uuid.NewString(),
		done:          make(chan struct{}),
	}
}

// Start 启动固定数量的执行槽和一个过期租约恢复循环，并等待进程停止。
func (w *ExecutionWorker) Start(ctx context.Context) error {
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
		w.recoverExpiredExecutions(ctx)
	}()
	for slot := 1; slot <= w.concurrency; slot++ {
		workerID := fmt.Sprintf("%s/%d", w.instanceID, slot)
		go func() {
			defer group.Done()
			w.execute(ctx, workerID, slot)
		}()
	}
	group.Wait()
	return nil
}

// execute 串行使用一个执行槽；多个槽之间通过数据库原子领取实现并发。
func (w *ExecutionWorker) execute(ctx context.Context, workerID string, slot int) {
	for {
		if ctx.Err() != nil {
			return
		}

		claimed, err := w.executor.ExecuteNext(ctx, workerID, w.leaseDuration)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			w.logger.Error("assistant execution failed", "slot", slot, "err", err)
			if !w.wait(ctx, executionErrorDelay) {
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

// recoverExpiredExecutions 独立回收已经失去租约持有者的执行，避免每个执行槽重复扫描。
func (w *ExecutionWorker) recoverExpiredExecutions(ctx context.Context) {
	for {
		count, err := w.executor.RecoverExpiredExecutions(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			w.logger.Error("recover expired assistant executions failed", "err", err)
			if !w.wait(ctx, executionErrorDelay) {
				return
			}
			continue
		}
		if count > 0 {
			w.logger.Warn("expired assistant executions marked as failed", "count", count)
		}
		if !w.wait(ctx, w.leaseDuration) {
			return
		}
	}
}

// Stop 取消正在执行的模型调用，并等待执行状态完成收敛。
func (w *ExecutionWorker) Stop(ctx context.Context) error {
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
		return fmt.Errorf("stop assistant execution worker: %w", ctx.Err())
	}
}

func (w *ExecutionWorker) wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
