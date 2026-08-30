package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	agentbiz "github.com/lgc202/ingate/internal/assistant/biz/agent"
)

// eventRecorder 把 Agent 过程事件翻译成执行记录和短期流式通知。
// Agent 只描述发生了什么；哪些事件需要持久化由执行编排层统一决定。
type eventRecorder struct {
	store         ExecutorStore
	events        EventStore
	executionID   string
	workerID      string
	leaseDuration time.Duration
}

// Emit 是 Agent 事件协议与执行状态机之间的唯一入口。
func (r *eventRecorder) Emit(ctx context.Context, event agentbiz.Event) error {
	switch event := event.(type) {
	case agentbiz.ModelSelected:
		return r.selectModel(ctx, event)
	case agentbiz.ModelCallStarted:
		return r.startStep(ctx, event.CallID, event.Model, StepKindModelCall)
	case agentbiz.ModelCallCompleted:
		return r.completeStep(ctx, event.CallID, StepKindModelCall, event.Summary)
	case agentbiz.ToolCallStarted:
		return r.startStep(ctx, event.CallID, event.Tool, StepKindToolCall)
	case agentbiz.ToolCallCompleted:
		return r.completeStep(ctx, event.CallID, StepKindToolCall, event.Summary)
	case agentbiz.ToolCallFailed:
		return r.failToolStep(ctx, event)
	case agentbiz.ReasoningDelta:
		r.appendStreamEvent(ctx, StreamEvent{Type: EventReasoningDelta, Data: event.Content})
		return nil
	case agentbiz.ContentDelta:
		r.appendStreamEvent(ctx, StreamEvent{Type: EventContentDelta, Data: event.Content})
		return nil
	default:
		return fmt.Errorf("unsupported assistant agent event %T", event)
	}
}

func newEventRecorder(
	store ExecutorStore,
	events EventStore,
	executionID string,
	workerID string,
	leaseDuration time.Duration,
) *eventRecorder {
	return &eventRecorder{
		store:         store,
		events:        events,
		executionID:   executionID,
		workerID:      workerID,
		leaseDuration: leaseDuration,
	}
}

func (r *eventRecorder) selectModel(
	ctx context.Context,
	event agentbiz.ModelSelected,
) error {
	if err := r.store.SetExecutionModel(ctx, r.executionID, r.workerID, event.Model); err != nil {
		return executionRecordError("set execution model", err)
	}

	// 选模发生在第一次远端请求之前。这里立即续租一次，既确认当前实例仍持有任务，
	// 也让已经到达的取消请求在产生模型费用前生效。
	cancelRequested, err := r.store.RenewExecutionLease(
		ctx,
		r.executionID,
		r.workerID,
		r.leaseDuration,
	)
	if err != nil {
		return executionRecordError("renew execution lease before model call", err)
	}
	if cancelRequested {
		return ErrCancellation
	}

	// Redis 只承载实时通知。MySQL 中的模型和执行状态才是可恢复事实，
	// 因此通知暂时不可用时仍允许任务继续并最终通过普通查询取得结果。
	r.appendStreamEvent(ctx, StreamEvent{Type: EventStarted, Data: r.executionID})
	return nil
}

func (r *eventRecorder) startStep(
	ctx context.Context,
	callID string,
	name string,
	kind StepKind,
) error {
	err := r.store.StartExecutionStep(ctx, r.executionID, r.workerID, Step{
		ID:     uuid.NewString(),
		Kind:   kind,
		Name:   name,
		CallID: callID,
	})
	if err != nil {
		return executionRecordError("start execution step", err)
	}
	return nil
}

func (r *eventRecorder) completeStep(
	ctx context.Context,
	callID string,
	kind StepKind,
	summary string,
) error {
	if err := r.store.CompleteExecutionStep(
		ctx,
		r.executionID,
		r.workerID,
		callID,
		kind,
		summary,
	); err != nil {
		return executionRecordError("complete execution step", err)
	}
	return nil
}

func (r *eventRecorder) failToolStep(
	ctx context.Context,
	event agentbiz.ToolCallFailed,
) error {
	if err := r.store.FailExecutionStep(
		ctx,
		r.executionID,
		r.workerID,
		event.CallID,
		StepKindToolCall,
		FailureToolUnavailable,
	); err != nil {
		return executionRecordError("fail tool call", err)
	}
	return nil
}

func (r *eventRecorder) appendStreamEvent(ctx context.Context, event StreamEvent) {
	// 事件流有过期时间，也允许客户端通过 MySQL 查询最终状态，因此这里有意降级为
	// 尽力写入。错误会由 Redis 连接层的健康状态暴露，不能反向改变执行终态。
	_, _ = r.events.Append(ctx, r.executionID, event)
}

func executionRecordError(operation string, err error) error {
	return errors.Join(
		errExecutionRecordUnavailable,
		fmt.Errorf("%s: %w", operation, err),
	)
}
