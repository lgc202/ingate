package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	agentbiz "github.com/lgc202/ingate/internal/assistant/biz/agent"
)

const (
	modelHistoryLimit = 200
	completionTimeout = 5 * time.Second
)

var errExecutionFinished = errors.New("assistant execution finished")

// Executor 负责领取任务、调用 Agent，并将执行过程收敛为持久终态。
// 它只由后台执行消费者调用，不进入 HTTP 请求链路。
type Executor struct {
	store  ExecutorStore
	events EventStore
	agent  Agent
}

// NewExecutor 创建后台执行编排器。
func NewExecutor(store ExecutorStore, events EventStore, agent Agent) *Executor {
	return &Executor{store: store, events: events, agent: agent}
}

// ExecuteNext 领取并执行一条排队任务。返回 false 表示当前没有待执行任务。
//
// 领取与终态提交都校验 claimantID。模型调用不持有数据库事务，长耗时执行依靠租约续期，
// 因此多个 Assistant 实例可以并发领取任务而不会重复提交同一次执行。
func (e *Executor) ExecuteNext(
	ctx context.Context,
	claimantID string,
	leaseDuration time.Duration,
) (bool, error) {
	claimed, found, err := e.store.ClaimExecution(ctx, claimantID, leaseDuration)
	if err != nil || !found {
		return false, err
	}
	if err := e.executeClaim(ctx, claimed, claimantID, leaseDuration); err != nil {
		return true, fmt.Errorf("execute assistant task %s: %w", claimed.ID, err)
	}
	return true, nil
}

// RecoverExpiredExecutions 将失去执行实例的任务收敛为失败终态。
// 不自动重试模型或工具调用，避免未来具有副作用的操作被重复执行。
func (e *Executor) RecoverExpiredExecutions(ctx context.Context) (int64, error) {
	return e.store.FailExpiredExecutions(ctx)
}

func (e *Executor) executeClaim(
	ctx context.Context,
	claim Claim,
	claimantID string,
	leaseDuration time.Duration,
) error {
	lease := e.startExecutionLease(ctx, claim, claimantID, leaseDuration)
	executionErr := e.invokeAgent(lease.ctx, claim, claimantID, leaseDuration)
	cause, leaseErr := lease.stop()

	// 执行结束、用户取消、实例停止和租约丢失共享同一个 context，但对应不同的
	// 持久化结果。这里只解释停止原因，不再参与模型循环或租约续期。
	switch {
	case errors.Is(cause, ErrCancellation):
		return e.finishCancellation(ctx, claim, claimantID)
	case errors.Is(cause, ErrLeaseLost):
		return ErrLeaseLost
	case leaseErr != nil:
		return leaseErr
	case errors.Is(cause, context.Canceled), errors.Is(cause, context.DeadlineExceeded):
		return e.finishFailure(ctx, claim, claimantID, FailureWorkerStopped, cause)
	case errors.Is(cause, errExecutionFinished):
		if errors.Is(executionErr, ErrCancellation) {
			return e.finishCancellation(ctx, claim, claimantID)
		}
		return executionErr
	default:
		return fmt.Errorf("unexpected assistant execution cancellation cause: %w", cause)
	}
}

func (e *Executor) invokeAgent(
	ctx context.Context,
	claim Claim,
	claimantID string,
	leaseDuration time.Duration,
) error {
	history, err := e.store.ListRecentMessages(
		ctx,
		claim.ActorID,
		claim.ConversationID,
		modelHistoryLimit,
	)
	if err != nil {
		return e.finishFailure(
			ctx,
			claim,
			claimantID,
			FailureInternal,
			fmt.Errorf("load conversation history: %w", err),
		)
	}

	// 事件写入器是 Agent 与任务状态机之间的唯一桥梁。Agent 无需知道租约、
	// MySQL 步骤表或 Redis SSE 的存在，Executor 也不需要理解 Eino 回调。
	recorder := newEventRecorder(
		e.store,
		e.events,
		claim.ID,
		claimantID,
		leaseDuration,
	)
	result, err := e.agent.Execute(
		ctx,
		newAgentRequest(history),
		recorder,
	)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		// 租约和取消是状态机信号，不能由已经失去租约的实例提交失败终态。
		if errors.Is(err, ErrLeaseLost) || errors.Is(err, ErrCancellation) {
			return err
		}
		return e.finishFailure(ctx, claim, claimantID, failureCode(err), err)
	}
	message, err := e.store.CompleteExecution(
		ctx,
		claim.ActorID,
		claim.ID,
		claimantID,
		Completion{
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
	_, _ = e.events.Append(ctx, claim.ID, StreamEvent{Type: EventCompleted, Data: message.ID})
	return nil
}

// executionLease 管理一条已领取任务的执行上下文和续租协程。
// 它不是持久化租约本身；MySQL 中的 worker_id 与 lease_expires_at 才是归属事实。
type executionLease struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	stopped chan error
}

func (e *Executor) startExecutionLease(
	ctx context.Context,
	claim Claim,
	claimantID string,
	leaseDuration time.Duration,
) *executionLease {
	executionCtx, cancel := context.WithCancelCause(ctx)
	lease := &executionLease{
		ctx:     executionCtx,
		cancel:  cancel,
		stopped: make(chan error, 1),
	}
	go func() {
		lease.stopped <- e.watchLease(
			executionCtx,
			cancel,
			claim.ID,
			claimantID,
			leaseDuration,
		)
	}()
	return lease
}

// stop 通知续租协程退出，并返回最终取消原因与续租错误。
// cancel cause 只接受第一次写入，因此租约丢失和用户取消不会被正常结束覆盖。
func (l *executionLease) stop() (error, error) {
	l.cancel(errExecutionFinished)
	leaseErr := <-l.stopped
	return context.Cause(l.ctx), leaseErr
}

// watchLease 在模型执行期间续租，并把用户取消请求转换为同一条执行上下文的取消原因。
func (e *Executor) watchLease(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	executionID string,
	claimantID string,
	leaseDuration time.Duration,
) error {
	// 一秒上限让同一轮询同时承担续租和取消检测，避免再引入一条取消消息通道。
	interval := min(leaseDuration/3, time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			cancelRequested, err := e.store.RenewExecutionLease(
				ctx,
				executionID,
				claimantID,
				leaseDuration,
			)
			if err != nil {
				cancel(err)
				return err
			}
			if cancelRequested {
				cancel(ErrCancellation)
				return ErrCancellation
			}
		}
	}
}

// finishCancellation 使用不受原执行取消影响的短超时上下文提交取消终态。
func (e *Executor) finishCancellation(
	ctx context.Context,
	claim Claim,
	claimantID string,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completionTimeout)
	defer cancel()
	if err := e.store.FinishExecutionCancellation(cleanupCtx, claim.ID, claimantID); err != nil {
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
	claimantID string,
	code FailureCode,
	cause error,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completionTimeout)
	defer cancel()
	if err := e.store.FailExecution(
		cleanupCtx,
		claim.ActorID,
		claim.ID,
		claimantID,
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
	case errors.Is(err, agentbiz.ErrToolUnavailable):
		return FailureToolUnavailable
	case errors.Is(err, errExecutionRecordUnavailable):
		return FailureInternal
	default:
		return FailureModelUnavailable
	}
}
