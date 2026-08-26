package execution

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const modelHistoryLimit = 200

var errExecutionFinished = errors.New("assistant execution finished")

// Executor 负责领取任务、调用 Agent，并将执行过程收敛为持久终态。
// 它只由后台 Worker 调用，不进入 HTTP 请求链路。
type Executor struct {
	store  Store
	events EventStore
	agent  Agent
}

// NewExecutor 创建后台执行编排器。
func NewExecutor(store Store, events EventStore, agent Agent) *Executor {
	return &Executor{store: store, events: events, agent: agent}
}

// ExecuteNext 领取并执行一条排队任务。返回 false 表示当前没有待执行任务。
//
// 领取与终态提交都校验 workerID。模型调用不持有数据库事务，长耗时执行依靠租约续期，
// 因此多个 Assistant 实例可以并发领取任务而不会重复提交同一次执行。
func (e *Executor) ExecuteNext(
	ctx context.Context,
	workerID string,
	leaseDuration time.Duration,
) (bool, error) {
	claimed, found, err := e.store.ClaimExecution(ctx, workerID, leaseDuration)
	if err != nil || !found {
		return false, err
	}
	if err := e.execute(ctx, claimed, workerID, leaseDuration); err != nil {
		return true, fmt.Errorf("execute assistant task %s: %w", claimed.ID, err)
	}
	return true, nil
}

// RecoverExpiredExecutions 将失去执行实例的任务收敛为失败终态。
// 不自动重试模型或工具调用，避免未来具有副作用的操作被重复执行。
func (e *Executor) RecoverExpiredExecutions(ctx context.Context) (int64, error) {
	return e.store.FailExpiredExecutions(ctx)
}

func (e *Executor) execute(
	ctx context.Context,
	claimed ClaimedExecution,
	workerID string,
	leaseDuration time.Duration,
) error {
	executionCtx, cancelExecution := context.WithCancelCause(ctx)
	leaseDone := make(chan error, 1)
	go func() {
		leaseDone <- e.keepLease(
			executionCtx,
			cancelExecution,
			claimed.ID,
			workerID,
			leaseDuration,
		)
	}()

	executionErr := e.generate(executionCtx, claimed, workerID, leaseDuration)
	cancelExecution(errExecutionFinished)
	leaseErr := <-leaseDone
	cause := context.Cause(executionCtx)

	switch {
	case errors.Is(cause, ErrCancellation):
		return e.finishCancellation(ctx, claimed, workerID)
	case errors.Is(cause, ErrLeaseLost):
		return ErrLeaseLost
	case leaseErr != nil:
		return leaseErr
	case errors.Is(cause, context.Canceled), errors.Is(cause, context.DeadlineExceeded):
		return e.finishFailure(ctx, claimed, workerID, FailureWorkerStopped, cause)
	case errors.Is(cause, errExecutionFinished):
		if errors.Is(executionErr, ErrCancellation) {
			return e.finishCancellation(ctx, claimed, workerID)
		}
		return executionErr
	default:
		return fmt.Errorf("unexpected assistant execution cancellation cause: %w", cause)
	}
}

func (e *Executor) generate(
	ctx context.Context,
	claimed ClaimedExecution,
	workerID string,
	leaseDuration time.Duration,
) error {
	history, err := e.store.ListRecentMessages(
		ctx,
		claimed.ActorID,
		claimed.ConversationID,
		modelHistoryLimit,
	)
	if err != nil {
		return e.finishFailure(
			ctx,
			claimed,
			workerID,
			FailureInternal,
			fmt.Errorf("load conversation history: %w", err),
		)
	}

	request := AgentRequest{
		Messages: history,
		Recorder: newStepRecorder(e.store, claimed.ID, workerID),
		SelectModel: func(ctx context.Context, model string) error {
			if err := e.store.SetExecutionModel(ctx, claimed.ID, workerID, model); err != nil {
				return err
			}
			cancelRequested, err := e.store.RenewExecutionLease(
				ctx,
				claimed.ID,
				workerID,
				leaseDuration,
			)
			if err != nil {
				return err
			}
			if cancelRequested {
				return ErrCancellation
			}
			return e.appendEvent(ctx, claimed.ID, StreamEvent{
				Type: EventStarted,
				Data: claimed.ID,
			})
		},
		Emit: func(delta Delta) error {
			eventType := EventContentDelta
			if delta.Type == DeltaReasoning {
				eventType = EventReasoningDelta
			}
			return e.appendEvent(ctx, claimed.ID, StreamEvent{
				Type: eventType,
				Data: delta.Content,
			})
		},
	}

	result, err := e.agent.Execute(ctx, request)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		// 租约和取消是状态机信号，不能由已经失去租约的实例提交失败终态。
		if errors.Is(err, ErrLeaseLost) || errors.Is(err, ErrCancellation) {
			return err
		}
		return e.finishFailure(ctx, claimed, workerID, failureCode(err), err)
	}
	message, err := e.store.CompleteExecution(
		ctx,
		claimed.ActorID,
		claimed.ID,
		workerID,
		AgentResult{
			Content:          result.Content,
			ReasoningContent: result.ReasoningContent,
		},
	)
	if err != nil {
		if errors.Is(err, ErrCancellation) {
			return ErrCancellation
		}
		return fmt.Errorf("complete assistant execution: %w", err)
	}
	// MySQL 已提交成功终态；Redis 只负责短期通知，写入失败不能把成功任务误报为失败。
	_, _ = e.events.Append(ctx, claimed.ID, StreamEvent{Type: EventCompleted, Data: message.ID})
	return nil
}
