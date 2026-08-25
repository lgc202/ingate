package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	runbiz "github.com/lgc202/ingate/internal/assistant/biz/run"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// CompleteRun 原子地保存模型最终回复并把当前实例持有的 Run 标记为成功。
func (s *Store) CompleteRun(
	ctx context.Context,
	actorID string,
	runID string,
	workerID string,
	result runbiz.Result,
) (conversation.Message, error) {
	run, err := s.queries.GetRun(ctx, db.GetRunParams{ID: runID, ActorID: actorID})
	if err != nil {
		return conversation.Message{}, runNotFound(err)
	}
	var message conversation.Message
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: run.ConversationID, ActorID: actorID,
		}); err != nil {
			return runNotFound(err)
		}
		lockedRun, err := queries.GetRunForUpdate(ctx, db.GetRunForUpdateParams{ID: runID, ActorID: actorID})
		if err != nil {
			return runNotFound(err)
		}
		if lockedRun.State != runStateRunning || lockedRun.WorkerID != workerID {
			return runbiz.ErrLeaseLost
		}
		if lockedRun.CancellationRequested {
			return runbiz.ErrCancellation
		}

		now := time.Now().UTC()
		message = conversation.Message{
			ID:               uuid.NewString(),
			ConversationID:   lockedRun.ConversationID,
			RunID:            lockedRun.ID,
			Role:             conversation.RoleAssistant,
			Content:          result.Content,
			ReasoningContent: result.ReasoningContent,
			CreatedAt:        now,
		}
		if err := queries.CreateMessage(ctx, db.CreateMessageParams{
			ID:               message.ID,
			ConversationID:   message.ConversationID,
			RunID:            message.RunID,
			Role:             messageRoleAssistant,
			Content:          message.Content,
			ReasoningContent: message.ReasoningContent,
			CreatedAt:        message.CreatedAt,
		}); err != nil {
			return fmt.Errorf("create assistant message: %w", err)
		}
		rows, err := queries.CompleteRun(ctx, db.CompleteRunParams{
			FinishedAt: sql.NullTime{Time: now, Valid: true}, ID: runID, WorkerID: workerID,
		})
		if err != nil {
			return fmt.Errorf("complete assistant run: %w", err)
		}
		if rows != 1 {
			return runbiz.ErrLeaseLost
		}
		if err := queries.TouchConversation(ctx, db.TouchConversationParams{
			UpdatedAt: now, ID: lockedRun.ConversationID, ActorID: actorID,
		}); err != nil {
			return fmt.Errorf("update conversation activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return conversation.Message{}, fmt.Errorf("complete assistant run transaction: %w", err)
	}
	return message, nil
}

// FailRun 保存稳定错误码，并且只允许当前租约持有者结束 Run。
func (s *Store) FailRun(
	ctx context.Context,
	actorID string,
	runID string,
	workerID string,
	errorCode runbiz.FailureCode,
) error {
	run, err := s.queries.GetRun(ctx, db.GetRunParams{ID: runID, ActorID: actorID})
	if err != nil {
		return runNotFound(err)
	}
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: run.ConversationID, ActorID: actorID,
		}); err != nil {
			return runNotFound(err)
		}
		lockedRun, err := queries.GetRunForUpdate(ctx, db.GetRunForUpdateParams{ID: runID, ActorID: actorID})
		if err != nil {
			return runNotFound(err)
		}
		if lockedRun.State != runStateRunning || lockedRun.WorkerID != workerID {
			return runbiz.ErrLeaseLost
		}
		if lockedRun.CancellationRequested {
			return runbiz.ErrCancellation
		}

		now := time.Now().UTC()
		if err := queries.FailRunningRunItems(ctx, db.FailRunningRunItemsParams{
			ErrorCode:  string(errorCode),
			FinishedAt: sql.NullTime{Time: now, Valid: true},
			RunID:      runID,
		}); err != nil {
			return fmt.Errorf("fail assistant run items: %w", err)
		}
		rows, err := queries.FailRun(ctx, db.FailRunParams{
			ErrorCode: string(errorCode), FinishedAt: sql.NullTime{Time: now, Valid: true},
			ID: runID, WorkerID: workerID,
		})
		if err != nil {
			return fmt.Errorf("fail assistant run: %w", err)
		}
		if rows != 1 {
			return runbiz.ErrLeaseLost
		}
		if err := queries.TouchConversation(ctx, db.TouchConversationParams{
			UpdatedAt: now, ID: lockedRun.ConversationID, ActorID: actorID,
		}); err != nil {
			return fmt.Errorf("update conversation activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("fail assistant run transaction: %w", err)
	}
	return nil
}

// FinishRunCancellation 由持有租约的实例确认模型调用已经停止后写入取消终态。
func (s *Store) FinishRunCancellation(ctx context.Context, runID, workerID string) error {
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		now := time.Now().UTC()
		rows, err := queries.FinishRunCancellation(ctx, db.FinishRunCancellationParams{
			FinishedAt: sql.NullTime{Time: now, Valid: true}, ID: runID, WorkerID: workerID,
		})
		if err != nil {
			return fmt.Errorf("finish assistant run cancellation: %w", err)
		}
		if rows != 1 {
			return runbiz.ErrLeaseLost
		}
		if err := queries.CancelRunningRunItems(ctx, db.CancelRunningRunItemsParams{
			FinishedAt: sql.NullTime{Time: now, Valid: true},
			RunID:      runID,
		}); err != nil {
			return fmt.Errorf("cancel assistant run items: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("finish assistant run cancellation transaction: %w", err)
	}
	return nil
}
