package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	agentbiz "github.com/lgc202/ingate/internal/assistant/biz/agent"
	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// PauseExecution 原子发布 Eino 根中断对应的审批项，并释放当前 Worker 租约。
func (s *Store) PauseExecution(
	ctx context.Context,
	actorID string,
	executionID string,
	workerID string,
	interruption agentbiz.ApprovalInterruption,
) error {
	request := interruption.Request
	if uuid.Validate(request.ChangeID) != nil || request.CallID == "" || len(request.CallID) > 128 ||
		uuid.Validate(interruption.InterruptID) != nil ||
		request.Summary == "" || strings.TrimSpace(request.Summary) != request.Summary ||
		len(request.Summary) > 1024 {
		return errors.New("assistant approval interruption contains invalid metadata")
	}
	if err := request.Proposal.Validate(); err != nil {
		return fmt.Errorf("validate interrupted assistant change: %w", err)
	}
	configuration, err := json.Marshal(request.Proposal)
	if err != nil {
		return fmt.Errorf("marshal interrupted assistant change: %w", err)
	}

	stored, err := s.queries.GetExecution(ctx, db.GetExecutionParams{ID: executionID, ActorID: actorID})
	if err != nil {
		return executionNotFound(err)
	}
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: stored.ConversationID, ActorID: actorID,
		}); err != nil {
			return executionNotFound(err)
		}
		locked, err := queries.GetExecutionForUpdate(ctx, db.GetExecutionForUpdateParams{
			ID: executionID, ActorID: actorID,
		})
		if err != nil {
			return executionNotFound(err)
		}
		if locked.State != executionStateRunning || locked.WorkerID != workerID {
			return execution.ErrLeaseLost
		}
		if locked.CancellationRequested {
			return execution.ErrCancellation
		}
		checkpoint, err := queries.GetCheckpoint(ctx, executionID)
		if err != nil {
			return fmt.Errorf("read interrupted assistant checkpoint: %w", err)
		}
		if len(checkpoint) == 0 {
			return errors.New("interrupted assistant checkpoint is empty")
		}
		rows, err := queries.PauseExecutionStep(ctx, db.PauseExecutionStepParams{
			Summary:     "等待管理员审批",
			ExecutionID: executionID,
			CallID:      request.CallID,
			WorkerID:    workerID,
		})
		if err != nil {
			return fmt.Errorf("pause assistant change tool step: %w", err)
		}
		if rows != 1 {
			return execution.ErrLeaseLost
		}
		rows, err = queries.CreateProposedChange(ctx, db.CreateProposedChangeParams{
			ID:           request.ChangeID,
			CallID:       request.CallID,
			InterruptID:  interruption.InterruptID,
			Kind:         proposedChangeKindToDB(request.Proposal.Kind),
			Summary:      request.Summary,
			ProposalJson: configuration,
			ExecutionID:  executionID,
			WorkerID:     workerID,
		})
		if err != nil {
			return fmt.Errorf("create interrupted assistant change: %w", err)
		}
		if rows != 1 {
			return execution.ErrLeaseLost
		}
		rows, err = queries.PauseExecution(ctx, db.PauseExecutionParams{
			ID: executionID, WorkerID: workerID,
		})
		if err != nil {
			return fmt.Errorf("pause assistant execution: %w", err)
		}
		if rows != 1 {
			return execution.ErrLeaseLost
		}
		if err := queries.TouchConversation(ctx, db.TouchConversationParams{
			ID: stored.ConversationID, ActorID: actorID,
		}); err != nil {
			return fmt.Errorf("update conversation activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("pause assistant execution transaction: %w", err)
	}
	return nil
}

// CompleteExecution 原子地保存 Agent 最终回复，并把当前实例持有的执行标记为成功。
func (s *Store) CompleteExecution(
	ctx context.Context,
	actorID string,
	executionID string,
	workerID string,
	result execution.Completion,
) (conversation.Message, error) {
	stored, err := s.queries.GetExecution(ctx, db.GetExecutionParams{ID: executionID, ActorID: actorID})
	if err != nil {
		return conversation.Message{}, executionNotFound(err)
	}
	var message conversation.Message
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: stored.ConversationID, ActorID: actorID,
		}); err != nil {
			return executionNotFound(err)
		}
		locked, err := queries.GetExecutionForUpdate(ctx, db.GetExecutionForUpdateParams{
			ID: executionID, ActorID: actorID,
		})
		if err != nil {
			return executionNotFound(err)
		}
		if locked.State != executionStateRunning || locked.WorkerID != workerID {
			return execution.ErrLeaseLost
		}
		if locked.CancellationRequested {
			return execution.ErrCancellation
		}

		now, err := queries.CurrentTime(ctx)
		if err != nil {
			return fmt.Errorf("read MySQL time: %w", err)
		}
		message = conversation.Message{
			ID:               uuid.NewString(),
			ConversationID:   locked.ConversationID,
			ExecutionID:      locked.ID,
			Role:             conversation.RoleAssistant,
			Content:          result.Content,
			ReasoningContent: result.ReasoningContent,
			CreatedAt:        now,
		}
		if err := queries.CreateMessage(ctx, db.CreateMessageParams{
			ID:               message.ID,
			ConversationID:   message.ConversationID,
			ExecutionID:      message.ExecutionID,
			Role:             messageRoleAssistant,
			Content:          message.Content,
			ReasoningContent: message.ReasoningContent,
			CreatedAt:        message.CreatedAt,
		}); err != nil {
			return fmt.Errorf("create assistant message: %w", err)
		}
		rows, err := queries.CompleteExecution(ctx, db.CompleteExecutionParams{
			FinishedAt: sql.NullTime{Time: now, Valid: true},
			ID:         executionID,
			WorkerID:   workerID,
		})
		if err != nil {
			return fmt.Errorf("complete assistant execution: %w", err)
		}
		if rows != 1 {
			return execution.ErrLeaseLost
		}
		if err := queries.DeleteCheckpoint(ctx, executionID); err != nil {
			return fmt.Errorf("delete completed assistant checkpoint: %w", err)
		}
		if err := queries.TouchConversation(ctx, db.TouchConversationParams{
			ID:      locked.ConversationID,
			ActorID: actorID,
		}); err != nil {
			return fmt.Errorf("update conversation activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return conversation.Message{}, fmt.Errorf("complete assistant execution transaction: %w", err)
	}
	return message, nil
}

// FailExecution 保存稳定错误码，并且只允许当前租约持有者结束执行。
func (s *Store) FailExecution(
	ctx context.Context,
	actorID string,
	executionID string,
	workerID string,
	errorCode execution.FailureCode,
) error {
	stored, err := s.queries.GetExecution(ctx, db.GetExecutionParams{ID: executionID, ActorID: actorID})
	if err != nil {
		return executionNotFound(err)
	}
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: stored.ConversationID, ActorID: actorID,
		}); err != nil {
			return executionNotFound(err)
		}
		locked, err := queries.GetExecutionForUpdate(ctx, db.GetExecutionForUpdateParams{
			ID: executionID, ActorID: actorID,
		})
		if err != nil {
			return executionNotFound(err)
		}
		if locked.State != executionStateRunning || locked.WorkerID != workerID {
			return execution.ErrLeaseLost
		}
		if locked.CancellationRequested {
			return execution.ErrCancellation
		}
		if err := queries.MarkExecutionChangesOutcomeUnknown(ctx, db.MarkExecutionChangesOutcomeUnknownParams{
			ErrorCode:   string(changebiz.FailureOutcomeUnknown),
			ExecutionID: executionID,
		}); err != nil {
			return fmt.Errorf("mark interrupted configuration changes outcome unknown: %w", err)
		}

		if err := queries.FailRunningExecutionSteps(ctx, db.FailRunningExecutionStepsParams{
			ErrorCode:   string(errorCode),
			ExecutionID: executionID,
		}); err != nil {
			return fmt.Errorf("fail assistant execution steps: %w", err)
		}
		rows, err := queries.FailExecution(ctx, db.FailExecutionParams{
			ErrorCode: string(errorCode),
			ID:        executionID,
			WorkerID:  workerID,
		})
		if err != nil {
			return fmt.Errorf("fail assistant execution: %w", err)
		}
		if rows != 1 {
			return execution.ErrLeaseLost
		}
		if err := queries.DeleteCheckpoint(ctx, executionID); err != nil {
			return fmt.Errorf("delete failed assistant checkpoint: %w", err)
		}
		if err := queries.TouchConversation(ctx, db.TouchConversationParams{
			ID:      locked.ConversationID,
			ActorID: actorID,
		}); err != nil {
			return fmt.Errorf("update conversation activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("fail assistant execution transaction: %w", err)
	}
	return nil
}

// FinishExecutionCancellation 由持有租约的实例确认模型调用已经停止后写入取消终态。
func (s *Store) FinishExecutionCancellation(
	ctx context.Context,
	actorID string,
	executionID string,
	workerID string,
) error {
	stored, err := s.queries.GetExecution(ctx, db.GetExecutionParams{ID: executionID, ActorID: actorID})
	if err != nil {
		return executionNotFound(err)
	}
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: stored.ConversationID, ActorID: actorID,
		}); err != nil {
			return executionNotFound(err)
		}
		locked, err := queries.GetExecutionForUpdate(ctx, db.GetExecutionForUpdateParams{
			ID: executionID, ActorID: actorID,
		})
		if err != nil {
			return executionNotFound(err)
		}
		if locked.State != executionStateRunning || locked.WorkerID != workerID {
			return execution.ErrLeaseLost
		}
		if !locked.CancellationRequested {
			return execution.ErrStateConflict
		}
		if err := queries.MarkExecutionChangesOutcomeUnknown(ctx, db.MarkExecutionChangesOutcomeUnknownParams{
			ErrorCode:   string(changebiz.FailureOutcomeUnknown),
			ExecutionID: executionID,
		}); err != nil {
			return fmt.Errorf("mark cancelled configuration changes outcome unknown: %w", err)
		}

		rows, err := queries.FinishExecutionCancellation(ctx, db.FinishExecutionCancellationParams{
			ID:       executionID,
			WorkerID: workerID,
		})
		if err != nil {
			return fmt.Errorf("finish assistant execution cancellation: %w", err)
		}
		if rows != 1 {
			return execution.ErrLeaseLost
		}
		if err := queries.CancelRunningExecutionSteps(ctx, executionID); err != nil {
			return fmt.Errorf("cancel assistant execution steps: %w", err)
		}
		if err := queries.DeleteCheckpoint(ctx, executionID); err != nil {
			return fmt.Errorf("delete cancelled assistant checkpoint: %w", err)
		}
		if err := queries.TouchConversation(ctx, db.TouchConversationParams{
			ID:      locked.ConversationID,
			ActorID: actorID,
		}); err != nil {
			return fmt.Errorf("update conversation activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("finish assistant execution cancellation transaction: %w", err)
	}
	return nil
}
