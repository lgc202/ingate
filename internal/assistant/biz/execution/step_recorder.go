package execution

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// stepRecorder 把 Eino 已经发生的模型与工具调用写入同一次执行。
// 它不保存工具参数和原始输出，避免执行追踪变成第二份敏感请求日志。
type stepRecorder struct {
	store       Store
	executionID string
	workerID    string
}

func newStepRecorder(store Store, executionID, workerID string) *stepRecorder {
	return &stepRecorder{store: store, executionID: executionID, workerID: workerID}
}

func (r *stepRecorder) ModelStarted(ctx context.Context, callID, model string) error {
	return r.start(ctx, callID, model, StepKindModelCall)
}

func (r *stepRecorder) ModelCompleted(ctx context.Context, callID, summary string) error {
	return r.complete(ctx, callID, StepKindModelCall, summary)
}

func (r *stepRecorder) ToolStarted(ctx context.Context, callID, tool string) error {
	return r.start(ctx, callID, tool, StepKindToolCall)
}

func (r *stepRecorder) ToolCompleted(ctx context.Context, callID, summary string) error {
	return r.complete(ctx, callID, StepKindToolCall, summary)
}

func (r *stepRecorder) ToolFailed(ctx context.Context, callID string) error {
	if err := r.store.FailExecutionStep(
		ctx,
		r.executionID,
		r.workerID,
		callID,
		StepKindToolCall,
		FailureToolUnavailable,
	); err != nil {
		return executionRecordError("fail tool call", err)
	}
	return nil
}

func (r *stepRecorder) start(
	ctx context.Context,
	callID string,
	name string,
	kind StepKind,
) error {
	_, err := r.store.StartExecutionStep(ctx, r.executionID, r.workerID, Step{
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

func (r *stepRecorder) complete(
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

func executionRecordError(operation string, err error) error {
	return errors.Join(
		errExecutionRecordUnavailable,
		fmt.Errorf("%s: %w", operation, err),
	)
}
