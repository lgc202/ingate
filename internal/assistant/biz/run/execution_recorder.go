package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// executionRecorder 把 Eino 已经发生的模型与工具调用写入同一个 Run。
// 它不保存工具参数和原始输出，避免执行追踪变成第二份敏感请求日志。
type executionRecorder struct {
	store    Store
	runID    string
	workerID string
}

func newExecutionRecorder(store Store, runID, workerID string) *executionRecorder {
	return &executionRecorder{store: store, runID: runID, workerID: workerID}
}

func (r *executionRecorder) ModelStarted(ctx context.Context, callID, model string) error {
	return r.start(ctx, callID, model, ItemKindModelCall)
}

func (r *executionRecorder) ModelCompleted(ctx context.Context, callID, summary string) error {
	return r.complete(ctx, callID, ItemKindModelCall, summary)
}

func (r *executionRecorder) ToolStarted(ctx context.Context, callID, tool string) error {
	return r.start(ctx, callID, tool, ItemKindToolCall)
}

func (r *executionRecorder) ToolCompleted(ctx context.Context, callID, summary string) error {
	return r.complete(ctx, callID, ItemKindToolCall, summary)
}

func (r *executionRecorder) ToolFailed(ctx context.Context, callID string) error {
	if err := r.store.FailRunItem(
		ctx,
		r.runID,
		r.workerID,
		callID,
		ItemKindToolCall,
		FailureToolUnavailable,
	); err != nil {
		return executionRecordError("fail tool call", err)
	}
	return nil
}

func (r *executionRecorder) start(
	ctx context.Context,
	callID string,
	name string,
	kind ItemKind,
) error {
	_, err := r.store.StartRunItem(ctx, r.runID, r.workerID, Item{
		ID:     uuid.NewString(),
		Kind:   kind,
		Name:   name,
		CallID: callID,
	})
	if err != nil {
		return executionRecordError("start execution item", err)
	}
	return nil
}

func (r *executionRecorder) complete(
	ctx context.Context,
	callID string,
	kind ItemKind,
	summary string,
) error {
	if err := r.store.CompleteRunItem(
		ctx,
		r.runID,
		r.workerID,
		callID,
		kind,
		summary,
	); err != nil {
		return executionRecordError("complete execution item", err)
	}
	return nil
}

func executionRecordError(operation string, err error) error {
	return errors.Join(
		errExecutionRecordUnavailable,
		fmt.Errorf("%s: %w", operation, err),
	)
}
