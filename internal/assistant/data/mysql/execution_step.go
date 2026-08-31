package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// 执行步骤的类型和状态使用小整数持久化，避免把可变展示文本写入索引字段。
const (
	executionStepKindModelCall uint8 = iota + 1
	executionStepKindToolCall
)

const (
	executionStepStateRunning uint8 = iota + 1
	executionStepStateCompleted
	executionStepStateFailed
	executionStepStateCancelled
	executionStepStateWaitingApproval
)

// StartExecutionStep 在当前 Worker 持有执行租约时追加一个运行中的执行步骤。
// 执行行锁同时保护 sequence 分配，使同一次执行并发追加工具步骤时仍保持稳定顺序。
func (s *Store) StartExecutionStep(
	ctx context.Context,
	executionID string,
	workerID string,
	step execution.Step,
) error {
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetExecutionForWorkerUpdate(ctx, db.GetExecutionForWorkerUpdateParams{
			ID: executionID, WorkerID: workerID,
		}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return execution.ErrLeaseLost
			}
			return fmt.Errorf("lock assistant execution for step: %w", err)
		}
		sequence, err := queries.NextExecutionStepSequence(ctx, executionID)
		if err != nil {
			return fmt.Errorf("allocate assistant execution step sequence: %w", err)
		}
		if err := queries.CreateExecutionStep(ctx, db.CreateExecutionStepParams{
			ID:          step.ID,
			ExecutionID: executionID,
			Sequence:    uint32(sequence),
			Kind:        executionStepKindToDB(step.Kind),
			State:       executionStepStateRunning,
			Name:        step.Name,
			CallID:      step.CallID,
			Summary:     "",
			ErrorCode:   "",
		}); err != nil {
			return fmt.Errorf("create assistant execution step: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("start assistant execution step transaction: %w", err)
	}
	return nil
}

// CompleteExecutionStep 只结束当前 Worker 实际执行成功的步骤。
// 执行终态不再批量补齐运行中步骤，避免把未完成的模型或工具调用伪装成成功。
func (s *Store) CompleteExecutionStep(
	ctx context.Context,
	executionID string,
	workerID string,
	callID string,
	kind execution.StepKind,
	summary string,
) error {
	rows, err := s.queries.CompleteExecutionStep(ctx, db.CompleteExecutionStepParams{
		Summary:     summary,
		ExecutionID: executionID,
		CallID:      callID,
		Kind:        executionStepKindToDB(kind),
		WorkerID:    workerID,
	})
	if err != nil {
		return fmt.Errorf("complete assistant execution step: %w", err)
	}
	if rows != 1 {
		return execution.ErrLeaseLost
	}
	return nil
}

// FailExecutionStep 记录工具调用等单个步骤的稳定失败码，
// 原始错误只沿调用链返回日志边界。
func (s *Store) FailExecutionStep(
	ctx context.Context,
	executionID string,
	workerID string,
	callID string,
	kind execution.StepKind,
	code execution.FailureCode,
) error {
	rows, err := s.queries.FailExecutionStep(ctx, db.FailExecutionStepParams{
		ErrorCode:   string(code),
		ExecutionID: executionID,
		CallID:      callID,
		Kind:        executionStepKindToDB(kind),
		WorkerID:    workerID,
	})
	if err != nil {
		return fmt.Errorf("fail assistant execution step: %w", err)
	}
	if rows != 1 {
		return execution.ErrLeaseLost
	}
	return nil
}

// ListExecutionSteps 按执行顺序返回指定用户可见的步骤。
func (s *Store) ListExecutionSteps(ctx context.Context, actorID, executionID string) ([]execution.Step, error) {
	if _, err := s.GetExecution(ctx, actorID, executionID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListExecutionSteps(ctx, db.ListExecutionStepsParams{
		ExecutionID: executionID,
		ActorID:     actorID,
	})
	if err != nil {
		return nil, fmt.Errorf("list assistant execution steps: %w", err)
	}
	steps := make([]execution.Step, 0, len(rows))
	for _, row := range rows {
		step, err := executionStepFromDB(row)
		if err != nil {
			return nil, fmt.Errorf("restore assistant execution step: %w", err)
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func executionStepFromDB(item db.AssistantAgentExecutionStep) (execution.Step, error) {
	if uuid.Validate(item.ID) != nil || uuid.Validate(item.ExecutionID) != nil ||
		item.Sequence == 0 || item.Name == "" ||
		item.CallID == "" || item.StartedAt.IsZero() {
		return execution.Step{}, fmt.Errorf("invalid stored assistant execution step %q", item.ID)
	}
	kind, err := executionStepKindFromDB(item.Kind)
	if err != nil {
		return execution.Step{}, err
	}
	state, err := executionStepStateFromDB(item.State)
	if err != nil {
		return execution.Step{}, err
	}
	result := execution.Step{
		ID:          item.ID,
		ExecutionID: item.ExecutionID,
		Sequence:    item.Sequence,
		Kind:        kind,
		State:       state,
		Name:        item.Name,
		CallID:      item.CallID,
		Summary:     item.Summary,
		ErrorCode:   execution.FailureCode(item.ErrorCode),
		StartedAt:   item.StartedAt,
	}
	if item.FinishedAt.Valid {
		result.FinishedAt = &item.FinishedAt.Time
	}
	if item.FinishedAt.Valid && item.FinishedAt.Time.Before(item.StartedAt) {
		return execution.Step{}, fmt.Errorf("assistant execution step %s contains invalid timestamps", item.ID)
	}
	switch result.State {
	case execution.StepStateRunning:
		if item.FinishedAt.Valid || result.Summary != "" || result.ErrorCode != "" {
			return execution.Step{}, fmt.Errorf("running assistant execution step %s is inconsistent", item.ID)
		}
	case execution.StepStateCompleted:
		if !item.FinishedAt.Valid || result.Summary == "" || result.ErrorCode != "" {
			return execution.Step{}, fmt.Errorf("completed assistant execution step %s is inconsistent", item.ID)
		}
	case execution.StepStateFailed:
		if !item.FinishedAt.Valid || result.Summary != "" || !validFailureCode(result.ErrorCode) {
			return execution.Step{}, fmt.Errorf("failed assistant execution step %s is inconsistent", item.ID)
		}
	case execution.StepStateCancelled:
		if !item.FinishedAt.Valid || result.Summary != "" || result.ErrorCode != "" {
			return execution.Step{}, fmt.Errorf("cancelled assistant execution step %s is inconsistent", item.ID)
		}
	case execution.StepStateWaitingApproval:
		if item.FinishedAt.Valid || result.Summary == "" || result.ErrorCode != "" {
			return execution.Step{}, fmt.Errorf("waiting assistant execution step %s is inconsistent", item.ID)
		}
	}
	return result, nil
}

func executionStepKindToDB(kind execution.StepKind) uint8 {
	switch kind {
	case execution.StepKindModelCall:
		return executionStepKindModelCall
	case execution.StepKindToolCall:
		return executionStepKindToolCall
	default:
		return 0
	}
}

func executionStepKindFromDB(kind uint8) (execution.StepKind, error) {
	switch kind {
	case executionStepKindModelCall:
		return execution.StepKindModelCall, nil
	case executionStepKindToolCall:
		return execution.StepKindToolCall, nil
	default:
		return "", fmt.Errorf("invalid assistant execution step kind %d", kind)
	}
}

func executionStepStateFromDB(state uint8) (execution.StepState, error) {
	switch state {
	case executionStepStateRunning:
		return execution.StepStateRunning, nil
	case executionStepStateCompleted:
		return execution.StepStateCompleted, nil
	case executionStepStateFailed:
		return execution.StepStateFailed, nil
	case executionStepStateCancelled:
		return execution.StepStateCancelled, nil
	case executionStepStateWaitingApproval:
		return execution.StepStateWaitingApproval, nil
	default:
		return "", fmt.Errorf("invalid assistant execution step state %d", state)
	}
}
