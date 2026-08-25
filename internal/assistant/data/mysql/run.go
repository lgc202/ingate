package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// Run 状态在数据库中使用小整数保存，状态机语义仍由业务层的可读常量表达。
const (
	runStateQueued uint8 = iota + 1
	runStateRunning
	runStateSucceeded
	runStateFailed
	runStateCancelled
)

// CreateRun 原子地保存用户消息并创建排队中的 Run。
// 锁定会话后检查活跃 Run，使多个 Assistant 实例并发创建时仍只会成功一个。
func (s *Store) CreateRun(
	ctx context.Context,
	actorID string,
	conversationID string,
	content string,
) (conversation.Run, error) {
	var run conversation.Run
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: conversationID, ActorID: actorID,
		}); err != nil {
			return mapNotFound(err)
		}
		active, err := queries.CountActiveRuns(ctx, conversationID)
		if err != nil {
			return fmt.Errorf("count active assistant runs: %w", err)
		}
		if active > 0 {
			return conversation.ErrRunRunning
		}

		now := time.Now().UTC()
		run = conversation.Run{
			ID:             uuid.NewString(),
			ConversationID: conversationID,
			State:          conversation.StateQueued,
			CreatedAt:      now,
		}
		if err := queries.CreateRun(ctx, db.CreateRunParams{
			ID: run.ID, ConversationID: run.ConversationID, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("create assistant run: %w", err)
		}
		if err := queries.CreateMessage(ctx, db.CreateMessageParams{
			ID:               uuid.NewString(),
			ConversationID:   conversationID,
			RunID:            run.ID,
			Role:             messageRoleUser,
			Content:          content,
			ReasoningContent: "",
			CreatedAt:        now,
		}); err != nil {
			return fmt.Errorf("create user message: %w", err)
		}
		if err := queries.TouchConversation(ctx, db.TouchConversationParams{
			UpdatedAt: now, ID: conversationID, ActorID: actorID,
		}); err != nil {
			return fmt.Errorf("update conversation activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return conversation.Run{}, fmt.Errorf("create assistant run transaction: %w", err)
	}
	return run, nil
}

// ClaimRun 使用 SKIP LOCKED 领取最早的排队 Run。
// 数据库锁只覆盖领取事务，模型调用期间由有期限的 worker_id 租约保护。
func (s *Store) ClaimRun(
	ctx context.Context,
	workerID string,
	leaseDuration time.Duration,
) (conversation.ClaimedRun, bool, error) {
	var claimed conversation.ClaimedRun
	found := false
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		row, err := queries.ClaimNextRun(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("select queued assistant run: %w", err)
		}
		now := time.Now().UTC()
		rows, err := queries.StartRun(ctx, db.StartRunParams{
			WorkerID:       workerID,
			LeaseExpiresAt: sql.NullTime{Time: now.Add(leaseDuration), Valid: true},
			StartedAt:      sql.NullTime{Time: now, Valid: true},
			ID:             row.ID,
		})
		if err != nil {
			return fmt.Errorf("start assistant run: %w", err)
		}
		if rows != 1 {
			return conversation.ErrRunStateConflict
		}
		claimed = conversation.ClaimedRun{
			Run: conversation.Run{
				ID:             row.ID,
				ConversationID: row.ConversationID,
				State:          conversation.StateRunning,
				CreatedAt:      row.CreatedAt,
				StartedAt:      &now,
			},
			ActorID: row.ActorID,
		}
		found = true
		return nil
	})
	if err != nil {
		return conversation.ClaimedRun{}, false, fmt.Errorf("claim assistant run transaction: %w", err)
	}
	return claimed, found, nil
}

// SetRunModel 记录当前租约实际选中的模型，排队阶段不提前固化在线配置。
func (s *Store) SetRunModel(ctx context.Context, runID, workerID, model string) error {
	rows, err := s.queries.SetRunModel(ctx, db.SetRunModelParams{
		Model: model, ID: runID, WorkerID: workerID,
	})
	if err != nil {
		return fmt.Errorf("set assistant run model: %w", err)
	}
	if rows != 1 {
		return conversation.ErrRunLeaseLost
	}
	return nil
}

// RenewRunLease 延长当前实例的租约，并返回用户是否请求取消。
func (s *Store) RenewRunLease(
	ctx context.Context,
	runID string,
	workerID string,
	leaseDuration time.Duration,
) (bool, error) {
	rows, err := s.queries.RenewRunLease(ctx, db.RenewRunLeaseParams{
		LeaseExpiresAt: sql.NullTime{Time: time.Now().UTC().Add(leaseDuration), Valid: true},
		ID:             runID,
		WorkerID:       workerID,
	})
	if err != nil {
		return false, fmt.Errorf("renew assistant run lease: %w", err)
	}
	if rows != 1 {
		return false, conversation.ErrRunLeaseLost
	}
	cancelRequested, err := s.queries.RunCancellationRequested(ctx, db.RunCancellationRequestedParams{
		ID: runID, WorkerID: workerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return false, conversation.ErrRunLeaseLost
	}
	if err != nil {
		return false, fmt.Errorf("read assistant run cancellation: %w", err)
	}
	return cancelRequested, nil
}

// CompleteRun 原子地保存模型最终回复并把当前实例持有的 Run 标记为成功。
func (s *Store) CompleteRun(
	ctx context.Context,
	actorID string,
	runID string,
	workerID string,
	result conversation.ModelResult,
) (conversation.Message, error) {
	run, err := s.queries.GetRun(ctx, db.GetRunParams{ID: runID, ActorID: actorID})
	if err != nil {
		return conversation.Message{}, mapNotFound(err)
	}
	var message conversation.Message
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: run.ConversationID, ActorID: actorID,
		}); err != nil {
			return mapNotFound(err)
		}
		lockedRun, err := queries.GetRunForUpdate(ctx, db.GetRunForUpdateParams{ID: runID, ActorID: actorID})
		if err != nil {
			return mapNotFound(err)
		}
		if lockedRun.State != runStateRunning || lockedRun.WorkerID != workerID {
			return conversation.ErrRunLeaseLost
		}
		if lockedRun.CancellationRequested {
			return conversation.ErrRunCancelled
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
			return conversation.ErrRunLeaseLost
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
	errorCode conversation.FailureCode,
) error {
	run, err := s.queries.GetRun(ctx, db.GetRunParams{ID: runID, ActorID: actorID})
	if err != nil {
		return mapNotFound(err)
	}
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: run.ConversationID, ActorID: actorID,
		}); err != nil {
			return mapNotFound(err)
		}
		lockedRun, err := queries.GetRunForUpdate(ctx, db.GetRunForUpdateParams{ID: runID, ActorID: actorID})
		if err != nil {
			return mapNotFound(err)
		}
		if lockedRun.State != runStateRunning || lockedRun.WorkerID != workerID {
			return conversation.ErrRunLeaseLost
		}
		if lockedRun.CancellationRequested {
			return conversation.ErrRunCancelled
		}

		now := time.Now().UTC()
		rows, err := queries.FailRun(ctx, db.FailRunParams{
			ErrorCode: string(errorCode), FinishedAt: sql.NullTime{Time: now, Valid: true},
			ID: runID, WorkerID: workerID,
		})
		if err != nil {
			return fmt.Errorf("fail assistant run: %w", err)
		}
		if rows != 1 {
			return conversation.ErrRunLeaseLost
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

// CancelRun 立即取消排队 Run；已经开始执行的 Run 只记录请求，由持有租约的实例终止模型调用。
func (s *Store) CancelRun(ctx context.Context, actorID, runID string) (conversation.Run, error) {
	storedRun, err := s.queries.GetRun(ctx, db.GetRunParams{ID: runID, ActorID: actorID})
	if err != nil {
		return conversation.Run{}, mapNotFound(err)
	}
	var result conversation.Run
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: storedRun.ConversationID, ActorID: actorID,
		}); err != nil {
			return mapNotFound(err)
		}
		lockedRun, err := queries.GetRunForUpdate(ctx, db.GetRunForUpdateParams{ID: runID, ActorID: actorID})
		if err != nil {
			return mapNotFound(err)
		}
		result, err = runFromDB(lockedRun)
		if err != nil {
			return fmt.Errorf("decode assistant run: %w", err)
		}
		switch result.State {
		case conversation.StateQueued:
			now := time.Now().UTC()
			rows, err := queries.CancelQueuedRun(ctx, db.CancelQueuedRunParams{
				FinishedAt: sql.NullTime{Time: now, Valid: true}, ID: runID,
			})
			if err != nil {
				return fmt.Errorf("cancel queued assistant run: %w", err)
			}
			if rows != 1 {
				return conversation.ErrRunStateConflict
			}
			result.State = conversation.StateCancelled
			result.FinishedAt = &now
		case conversation.StateRunning:
			if result.CancellationRequested {
				return nil
			}
			rows, err := queries.RequestRunCancellation(ctx, runID)
			if err != nil {
				return fmt.Errorf("request assistant run cancellation: %w", err)
			}
			if rows != 1 {
				return conversation.ErrRunStateConflict
			}
			result.CancellationRequested = true
		}
		return nil
	})
	if err != nil {
		return conversation.Run{}, fmt.Errorf("cancel assistant run transaction: %w", err)
	}
	return result, nil
}

// FinishRunCancellation 由持有租约的实例确认模型调用已经停止后写入取消终态。
func (s *Store) FinishRunCancellation(ctx context.Context, runID, workerID string) error {
	rows, err := s.queries.FinishRunCancellation(ctx, db.FinishRunCancellationParams{
		FinishedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true}, ID: runID, WorkerID: workerID,
	})
	if err != nil {
		return fmt.Errorf("finish assistant run cancellation: %w", err)
	}
	if rows != 1 {
		return conversation.ErrRunLeaseLost
	}
	return nil
}

// FailExpiredRuns 终止已经失去执行实例的 Run。
// 不自动重新排队，避免未来 Run 包含有副作用的工具调用时重复执行。
func (s *Store) FailExpiredRuns(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	rows, err := s.queries.FailExpiredRuns(ctx, db.FailExpiredRunsParams{
		ErrorCode:      string(conversation.FailureWorkerLost),
		FinishedAt:     sql.NullTime{Time: now, Valid: true},
		LeaseExpiresAt: sql.NullTime{Time: now, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("fail expired assistant runs: %w", err)
	}
	return rows, nil
}

// GetRun 按所有者和 Run ID 查询一次模型调用。
func (s *Store) GetRun(ctx context.Context, actorID, id string) (conversation.Run, error) {
	item, err := s.queries.GetRun(ctx, db.GetRunParams{ID: id, ActorID: actorID})
	if err != nil {
		return conversation.Run{}, mapNotFound(err)
	}
	result, err := runFromDB(item)
	if err != nil {
		return conversation.Run{}, fmt.Errorf("decode assistant run: %w", err)
	}
	return result, nil
}

func runFromDB(item db.AssistantRun) (conversation.Run, error) {
	state, err := runStateFromDB(item.State)
	if err != nil {
		return conversation.Run{}, err
	}
	result := conversation.Run{
		ID:                    item.ID,
		ConversationID:        item.ConversationID,
		State:                 state,
		Model:                 item.Model,
		ErrorCode:             conversation.FailureCode(item.ErrorCode),
		CancellationRequested: item.CancellationRequested,
		CreatedAt:             item.CreatedAt,
	}
	if item.StartedAt.Valid {
		result.StartedAt = &item.StartedAt.Time
	}
	if item.FinishedAt.Valid {
		result.FinishedAt = &item.FinishedAt.Time
	}
	return result, nil
}

func runStateFromDB(state uint8) (conversation.RunState, error) {
	switch state {
	case runStateQueued:
		return conversation.StateQueued, nil
	case runStateRunning:
		return conversation.StateRunning, nil
	case runStateSucceeded:
		return conversation.StateSucceeded, nil
	case runStateFailed:
		return conversation.StateFailed, nil
	case runStateCancelled:
		return conversation.StateCancelled, nil
	default:
		return "", fmt.Errorf("invalid assistant run state %d", state)
	}
}
