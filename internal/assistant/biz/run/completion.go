package run

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const completionTimeout = 5 * time.Second

func (s *Service) appendEvent(ctx context.Context, runID string, event StreamEvent) error {
	if _, err := s.events.Append(ctx, runID, event); err != nil {
		return errors.Join(errEventStoreUnavailable, fmt.Errorf("append assistant run event: %w", err))
	}
	return nil
}

// finishCancellation 使用不受原执行取消影响的短超时上下文提交取消终态。
func (s *Service) finishCancellation(ctx context.Context, claimed Claimed, workerID string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completionTimeout)
	defer cancel()
	if err := s.store.FinishRunCancellation(cleanupCtx, claimed.ID, workerID); err != nil {
		return fmt.Errorf("finish assistant run cancellation: %w", err)
	}
	// MySQL 是终态事实来源，取消事件写入失败只影响当前短期订阅。
	_, _ = s.events.Append(cleanupCtx, claimed.ID, StreamEvent{Type: EventCancelled})
	return nil
}

// finishFailure 先提交持久化终态，再尽力通知当前 SSE 订阅者。
func (s *Service) finishFailure(
	ctx context.Context,
	claimed Claimed,
	workerID string,
	code FailureCode,
	cause error,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completionTimeout)
	defer cancel()
	if err := s.store.FailRun(cleanupCtx, claimed.ActorID, claimed.ID, workerID, code); err != nil {
		if errors.Is(err, ErrCancellation) {
			return ErrCancellation
		}
		return errors.Join(cause, fmt.Errorf("mark assistant run failed: %w", err))
	}
	// 错误码已经持久化；Redis 不可用时仍可通过 Run 查询最终结果。
	_, _ = s.events.Append(cleanupCtx, claimed.ID, StreamEvent{Type: EventFailed, Data: string(code)})
	return cause
}

func failureCode(err error) FailureCode {
	switch {
	case errors.Is(err, errEventStoreUnavailable):
		return FailureEventStore
	case errors.Is(err, ErrToolUnavailable):
		return FailureToolUnavailable
	case errors.Is(err, errExecutionRecordUnavailable):
		return FailureInternal
	default:
		return FailureModelUnavailable
	}
}
