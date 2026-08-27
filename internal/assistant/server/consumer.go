package server

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

// ExecutionConsumer 从 MySQL 领取排队任务，并维持模型执行期间的实例租约。
//
// 一个进程包含固定数量的执行槽，不同会话可以并发执行。部署多个进程时，MySQL 的
// SKIP LOCKED 负责分配任务，领取者标识和租约阻止失联或过期实例提交结果。
type ExecutionConsumer struct {
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

// NewExecutionConsumer 创建由 Kratos 管理生命周期的后台执行服务。
func NewExecutionConsumer(
	config *conf.Worker,
	executor *execution.Executor,
	logger *slog.Logger,
) *ExecutionConsumer {
	return &ExecutionConsumer{
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
func (c *ExecutionConsumer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.lifecycleMu.Lock()
	c.cancel = cancel
	stopping := c.stopping
	c.lifecycleMu.Unlock()
	if stopping {
		cancel()
	}
	defer close(c.done)

	var group sync.WaitGroup
	group.Add(c.concurrency + 1)
	go func() {
		defer group.Done()
		c.recoverExpiredExecutions(ctx)
	}()
	for slot := 1; slot <= c.concurrency; slot++ {
		claimantID := fmt.Sprintf("%s/%d", c.instanceID, slot)
		go func() {
			defer group.Done()
			c.serveSlot(ctx, claimantID, slot)
		}()
	}
	group.Wait()
	return nil
}

// serveSlot 串行使用一个执行槽；多个槽之间通过数据库原子领取实现并发。
func (c *ExecutionConsumer) serveSlot(ctx context.Context, claimantID string, slot int) {
	for {
		if ctx.Err() != nil {
			return
		}

		handled, err := c.executor.ExecuteNext(ctx, claimantID, c.leaseDuration)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.logger.Error("assistant execution failed", "slot", slot, "err", err)
			if !c.pause(ctx, executionErrorDelay) {
				return
			}
			continue
		}
		if handled {
			continue
		}
		if !c.pause(ctx, c.pollInterval) {
			return
		}
	}
}

// recoverExpiredExecutions 独立回收已经失去租约持有者的执行，避免每个执行槽重复扫描。
func (c *ExecutionConsumer) recoverExpiredExecutions(ctx context.Context) {
	for {
		count, err := c.executor.RecoverExpiredExecutions(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.logger.Error("recover expired assistant executions failed", "err", err)
			if !c.pause(ctx, executionErrorDelay) {
				return
			}
			continue
		}
		if count > 0 {
			c.logger.Warn("expired assistant executions marked as failed", "count", count)
		}
		if !c.pause(ctx, c.leaseDuration) {
			return
		}
	}
}

// Stop 取消正在执行的模型调用，并等待执行状态完成收敛。
func (c *ExecutionConsumer) Stop(ctx context.Context) error {
	c.lifecycleMu.Lock()
	c.stopping = true
	cancel := c.cancel
	c.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop assistant execution consumer: %w", ctx.Err())
	}
}

func (c *ExecutionConsumer) pause(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
