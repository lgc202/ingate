package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	runbiz "github.com/lgc202/ingate/internal/assistant/biz/run"
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
) (runbiz.Run, error) {
	var run runbiz.Run
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: conversationID, ActorID: actorID,
		}); err != nil {
			return mapConversationNotFound(err)
		}
		active, err := queries.CountActiveRuns(ctx, conversationID)
		if err != nil {
			return fmt.Errorf("count active assistant runs: %w", err)
		}
		if active > 0 {
			return runbiz.ErrConversationBusy
		}

		now := time.Now().UTC()
		run = runbiz.Run{
			ID:             uuid.NewString(),
			ConversationID: conversationID,
			State:          runbiz.StateQueued,
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
		return runbiz.Run{}, fmt.Errorf("create assistant run transaction: %w", err)
	}
	return run, nil
}

// CancelRun 立即取消排队 Run；已经开始执行的 Run 只记录请求，由持有租约的实例终止模型调用。
func (s *Store) CancelRun(ctx context.Context, actorID, runID string) (runbiz.Run, error) {
	storedRun, err := s.queries.GetRun(ctx, db.GetRunParams{ID: runID, ActorID: actorID})
	if err != nil {
		return runbiz.Run{}, runNotFound(err)
	}
	var result runbiz.Run
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: storedRun.ConversationID, ActorID: actorID,
		}); err != nil {
			return runNotFound(err)
		}
		lockedRun, err := queries.GetRunForUpdate(ctx, db.GetRunForUpdateParams{ID: runID, ActorID: actorID})
		if err != nil {
			return runNotFound(err)
		}
		result, err = runFromDB(lockedRun)
		if err != nil {
			return fmt.Errorf("decode assistant run: %w", err)
		}
		switch result.State {
		case runbiz.StateQueued:
			now := time.Now().UTC()
			rows, err := queries.CancelQueuedRun(ctx, db.CancelQueuedRunParams{
				FinishedAt: sql.NullTime{Time: now, Valid: true}, ID: runID,
			})
			if err != nil {
				return fmt.Errorf("cancel queued assistant run: %w", err)
			}
			if rows != 1 {
				return runbiz.ErrStateConflict
			}
			result.State = runbiz.StateCancelled
			result.FinishedAt = &now
		case runbiz.StateRunning:
			if result.CancellationRequested {
				return nil
			}
			rows, err := queries.RequestRunCancellation(ctx, runID)
			if err != nil {
				return fmt.Errorf("request assistant run cancellation: %w", err)
			}
			if rows != 1 {
				return runbiz.ErrStateConflict
			}
			result.CancellationRequested = true
		}
		return nil
	})
	if err != nil {
		return runbiz.Run{}, fmt.Errorf("cancel assistant run transaction: %w", err)
	}
	return result, nil
}

// GetRun 按所有者和 Run ID 查询一次模型调用。
func (s *Store) GetRun(ctx context.Context, actorID, id string) (runbiz.Run, error) {
	item, err := s.queries.GetRun(ctx, db.GetRunParams{ID: id, ActorID: actorID})
	if err != nil {
		return runbiz.Run{}, runNotFound(err)
	}
	result, err := runFromDB(item)
	if err != nil {
		return runbiz.Run{}, fmt.Errorf("decode assistant run: %w", err)
	}
	return result, nil
}

func runFromDB(item db.AssistantRun) (runbiz.Run, error) {
	state, err := runStateFromDB(item.State)
	if err != nil {
		return runbiz.Run{}, err
	}
	result := runbiz.Run{
		ID:                    item.ID,
		ConversationID:        item.ConversationID,
		State:                 state,
		Model:                 item.Model,
		ErrorCode:             runbiz.FailureCode(item.ErrorCode),
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

func runStateFromDB(state uint8) (runbiz.State, error) {
	switch state {
	case runStateQueued:
		return runbiz.StateQueued, nil
	case runStateRunning:
		return runbiz.StateRunning, nil
	case runStateSucceeded:
		return runbiz.StateSucceeded, nil
	case runStateFailed:
		return runbiz.StateFailed, nil
	case runStateCancelled:
		return runbiz.StateCancelled, nil
	default:
		return "", fmt.Errorf("invalid assistant run state %d", state)
	}
}

func runNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return runbiz.ErrNotFound
	}
	return err
}
