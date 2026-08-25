package conversation

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	modelHistoryLimit = 200
	failureTimeout    = 5 * time.Second
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
	run ClaimedRun,
	workerID string,
	leaseDuration time.Duration,
) error {
	runCtx, cancelRun := context.WithCancelCause(ctx)
	leaseDone := make(chan error, 1)
	go func() {
		leaseDone <- s.keepLease(runCtx, cancelRun, run.ID, workerID, leaseDuration)
	}()

	executionErr := s.generate(runCtx, run, workerID, leaseDuration)
	cancelRun(errExecutionFinished)
	leaseErr := <-leaseDone
	cause := context.Cause(runCtx)

	switch {
	case errors.Is(cause, ErrRunCancelled):
		return s.finishCancellation(ctx, run, workerID)
	case errors.Is(cause, ErrRunLeaseLost):
		return ErrRunLeaseLost
	case leaseErr != nil:
		return leaseErr
	case errors.Is(cause, context.Canceled), errors.Is(cause, context.DeadlineExceeded):
		return s.finishFailure(ctx, run, workerID, FailureWorkerStopped, cause)
	case errors.Is(cause, errExecutionFinished):
		if errors.Is(executionErr, ErrRunCancelled) {
			return s.finishCancellation(ctx, run, workerID)
		}
		return executionErr
	default:
		return fmt.Errorf("unexpected assistant run cancellation cause: %w", cause)
	}
}

func (s *Service) generate(
	ctx context.Context,
	run ClaimedRun,
	workerID string,
	leaseDuration time.Duration,
) error {
	selectedModel, err := s.agent.Model(ctx)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		return s.finishFailure(ctx, run, workerID, failureCode(err), err)
	}
	if err := s.store.SetRunModel(ctx, run.ID, workerID, selectedModel.Name()); err != nil {
		return err
	}
	cancelRequested, err := s.store.RenewRunLease(ctx, run.ID, workerID, leaseDuration)
	if err != nil {
		return err
	}
	if cancelRequested {
		return ErrRunCancelled
	}
	if err := s.appendEvent(ctx, run.ID, StreamEvent{Type: EventRunStarted, Data: run.ID}); err != nil {
		return s.finishFailure(ctx, run, workerID, FailureEventStore, err)
	}
	history, err := s.store.ListRecentMessages(ctx, run.ActorID, run.ConversationID, modelHistoryLimit)
	if err != nil {
		return s.finishFailure(
			ctx, run, workerID, FailureInternal, fmt.Errorf("load conversation history: %w", err),
		)
	}
	result, err := selectedModel.Generate(ctx, history, func(delta ModelDelta) error {
		eventType := EventContentDelta
		if delta.Type == ModelDeltaReasoning {
			eventType = EventReasoningDelta
		}
		return s.appendEvent(ctx, run.ID, StreamEvent{Type: eventType, Data: delta.Content})
	})
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		return s.finishFailure(ctx, run, workerID, failureCode(err), err)
	}
	message, err := s.store.CompleteRun(ctx, run.ActorID, run.ID, workerID, result)
	if err != nil {
		if errors.Is(err, ErrRunCancelled) {
			return ErrRunCancelled
		}
		return fmt.Errorf("complete assistant run: %w", err)
	}
	if err := s.appendEvent(ctx, run.ID, StreamEvent{Type: EventRunCompleted, Data: message.ID}); err != nil {
		// MySQL 已经记录成功终态，Redis 事件缺失不能把 Run 回滚为失败。
		return fmt.Errorf("append completed assistant run event: %w", err)
	}
	return nil
}

func (s *Service) keepLease(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	runID string,
	workerID string,
	leaseDuration time.Duration,
) error {
	// 租约不必频繁续写，但用户取消不应等待一个较长的租约周期才生效。
	// 一秒上限让同一轮询同时承担续租和取消检测，避免再引入消息通道。
	interval := min(leaseDuration/3, time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			cancelRequested, err := s.store.RenewRunLease(ctx, runID, workerID, leaseDuration)
			if err != nil {
				cancel(err)
				return err
			}
			if cancelRequested {
				cancel(ErrRunCancelled)
				return ErrRunCancelled
			}
		}
	}
}

func (s *Service) appendEvent(ctx context.Context, runID string, event StreamEvent) error {
	if _, err := s.events.Append(ctx, runID, event); err != nil {
		return errors.Join(errEventStoreUnavailable, fmt.Errorf("append assistant run event: %w", err))
	}
	return nil
}

func (s *Service) finishCancellation(ctx context.Context, run ClaimedRun, workerID string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureTimeout)
	defer cancel()
	if err := s.store.FinishRunCancellation(cleanupCtx, run.ID, workerID); err != nil {
		return fmt.Errorf("finish assistant run cancellation: %w", err)
	}
	// MySQL 是终态事实来源，取消事件写入失败只影响当前短期订阅。
	_, _ = s.events.Append(cleanupCtx, run.ID, StreamEvent{Type: EventRunCancelled})
	return nil
}

func (s *Service) finishFailure(
	ctx context.Context,
	run ClaimedRun,
	workerID string,
	code FailureCode,
	cause error,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureTimeout)
	defer cancel()
	if err := s.store.FailRun(cleanupCtx, run.ActorID, run.ID, workerID, code); err != nil {
		if errors.Is(err, ErrRunCancelled) {
			return ErrRunCancelled
		}
		return errors.Join(cause, fmt.Errorf("mark assistant run failed: %w", err))
	}
	// 错误码已经持久化；Redis 不可用时仍可通过 Run 查询最终结果。
	_, _ = s.events.Append(cleanupCtx, run.ID, StreamEvent{Type: EventRunFailed, Data: string(code)})
	return cause
}

func failureCode(err error) FailureCode {
	if errors.Is(err, errEventStoreUnavailable) {
		return FailureEventStore
	}
	return FailureModelUnavailable
}
