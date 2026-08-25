package run

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	modelHistoryLimit = 200
)

var errExecutionFinished = errors.New("assistant run execution finished")

// ExecuteNext 领取并执行一条排队中的 Run。返回 false 表示当前没有待执行任务。
//
// 领取与终态提交都校验 worker_id。模型调用不持有数据库事务，长耗时执行通过租约续期，
// 因此多个 Assistant 实例可以并发领取任务而不会重复提交同一个 Run。
func (s *Service) ExecuteNext(
	ctx context.Context,
	workerID string,
	leaseDuration time.Duration,
) (bool, error) {
	claimed, found, err := s.store.ClaimRun(ctx, workerID, leaseDuration)
	if err != nil || !found {
		return false, err
	}
	if err := s.execute(ctx, claimed, workerID, leaseDuration); err != nil {
		return true, fmt.Errorf("execute assistant run %s: %w", claimed.ID, err)
	}
	return true, nil
}

// RecoverExpiredRuns 把失去执行实例的 Run 收敛为失败终态。
// 不自动重试模型或工具调用，避免未来具有副作用的操作被重复执行。
func (s *Service) RecoverExpiredRuns(ctx context.Context) (int64, error) {
	return s.store.FailExpiredRuns(ctx)
}

func (s *Service) execute(
	ctx context.Context,
	claimed Claimed,
	workerID string,
	leaseDuration time.Duration,
) error {
	runCtx, cancelRun := context.WithCancelCause(ctx)
	leaseDone := make(chan error, 1)
	go func() {
		leaseDone <- s.keepLease(runCtx, cancelRun, claimed.ID, workerID, leaseDuration)
	}()

	executionErr := s.generate(runCtx, claimed, workerID, leaseDuration)
	cancelRun(errExecutionFinished)
	leaseErr := <-leaseDone
	cause := context.Cause(runCtx)

	switch {
	case errors.Is(cause, ErrCancellation):
		return s.finishCancellation(ctx, claimed, workerID)
	case errors.Is(cause, ErrLeaseLost):
		return ErrLeaseLost
	case leaseErr != nil:
		return leaseErr
	case errors.Is(cause, context.Canceled), errors.Is(cause, context.DeadlineExceeded):
		return s.finishFailure(ctx, claimed, workerID, FailureWorkerStopped, cause)
	case errors.Is(cause, errExecutionFinished):
		if errors.Is(executionErr, ErrCancellation) {
			return s.finishCancellation(ctx, claimed, workerID)
		}
		return executionErr
	default:
		return fmt.Errorf("unexpected assistant run cancellation cause: %w", cause)
	}
}

func (s *Service) generate(
	ctx context.Context,
	claimed Claimed,
	workerID string,
	leaseDuration time.Duration,
) error {
	history, err := s.store.ListRecentMessages(
		ctx,
		claimed.ActorID,
		claimed.ConversationID,
		modelHistoryLimit,
	)
	if err != nil {
		return s.finishFailure(
			ctx,
			claimed,
			workerID,
			FailureInternal,
			fmt.Errorf("load conversation history: %w", err),
		)
	}

	request := AgentRequest{
		Messages: history,
		Recorder: newExecutionRecorder(s.store, claimed.ID, workerID),
		SelectModel: func(ctx context.Context, model string) error {
			if err := s.store.SetRunModel(ctx, claimed.ID, workerID, model); err != nil {
				return err
			}
			cancelRequested, err := s.store.RenewRunLease(
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
			return s.appendEvent(ctx, claimed.ID, StreamEvent{Type: EventStarted, Data: claimed.ID})
		},
		Emit: func(delta Delta) error {
			eventType := EventContentDelta
			if delta.Type == DeltaReasoning {
				eventType = EventReasoningDelta
			}
			return s.appendEvent(ctx, claimed.ID, StreamEvent{Type: eventType, Data: delta.Content})
		},
	}

	result, err := s.agent.Execute(ctx, request)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		// 租约和取消是 Run 状态机信号，不属于模型故障，也不能由已经失去租约的实例提交失败终态。
		if errors.Is(err, ErrLeaseLost) || errors.Is(err, ErrCancellation) {
			return err
		}
		return s.finishFailure(ctx, claimed, workerID, failureCode(err), err)
	}
	message, err := s.store.CompleteRun(ctx, claimed.ActorID, claimed.ID, workerID, result)
	if err != nil {
		if errors.Is(err, ErrCancellation) {
			return ErrCancellation
		}
		return fmt.Errorf("complete assistant run: %w", err)
	}
	// MySQL 已经提交成功终态；Redis 只负责短期流式通知，写入失败不能把成功 Run 误报为失败。
	_, _ = s.events.Append(ctx, claimed.ID, StreamEvent{Type: EventCompleted, Data: message.ID})
	return nil
}
