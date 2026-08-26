package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	agentprotocol "github.com/lgc202/ingate/internal/assistant/agent"
)

const completionTimeout = 5 * time.Second

// finishCancellation 使用不受原执行取消影响的短超时上下文提交取消终态。
func (e *Executor) finishCancellation(
	ctx context.Context,
	claim Claim,
	workerID string,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completionTimeout)
	defer cancel()
	if err := e.store.FinishExecutionCancellation(cleanupCtx, claim.ID, workerID); err != nil {
		return fmt.Errorf("finish assistant execution cancellation: %w", err)
	}
	// MySQL 是终态事实来源，取消事件写入失败只影响当前短期订阅。
	_, _ = e.events.Append(cleanupCtx, claim.ID, StreamEvent{Type: EventCancelled})
	return nil
}

// finishFailure 先提交持久化终态，再尽力通知当前 SSE 订阅者。
func (e *Executor) finishFailure(
	ctx context.Context,
	claim Claim,
	workerID string,
	code FailureCode,
	cause error,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completionTimeout)
	defer cancel()
	if err := e.store.FailExecution(
		cleanupCtx,
		claim.ActorID,
		claim.ID,
		workerID,
		code,
	); err != nil {
		if errors.Is(err, ErrCancellation) {
			return ErrCancellation
		}
		return errors.Join(cause, fmt.Errorf("mark assistant execution failed: %w", err))
	}
	// 错误码已经持久化；Redis 不可用时仍可查询最终结果。
	_, _ = e.events.Append(cleanupCtx, claim.ID, StreamEvent{Type: EventFailed, Data: string(code)})
	return cause
}

func failureCode(err error) FailureCode {
	switch {
	case errors.Is(err, agentprotocol.ErrToolUnavailable):
		return FailureToolUnavailable
	case errors.Is(err, errExecutionRecordUnavailable):
		return FailureInternal
	default:
		return FailureModelUnavailable
	}
}
