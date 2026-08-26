package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// Agent 执行状态在数据库中使用小整数保存，状态机语义仍由业务层的可读常量表达。
const (
	executionStateQueued uint8 = iota + 1
	executionStateRunning
	executionStateSucceeded
	executionStateFailed
	executionStateCancelled
)

// CreateExecution 原子地保存用户消息并创建排队中的 Agent 执行。
// 锁定会话后检查活跃执行，使多个 Assistant 实例并发创建时仍只会成功一个。
func (s *Store) CreateExecution(
	ctx context.Context,
	actorID string,
	conversationID string,
	content string,
) (execution.AgentExecution, error) {
	var created execution.AgentExecution
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: conversationID, ActorID: actorID,
		}); err != nil {
			return mapConversationNotFound(err)
		}
		active, err := queries.CountActiveExecutions(ctx, conversationID)
		if err != nil {
			return fmt.Errorf("count active assistant executions: %w", err)
		}
		if active > 0 {
			return execution.ErrConversationBusy
		}

		now := time.Now().UTC()
		created = execution.AgentExecution{
			ID:             uuid.NewString(),
			ConversationID: conversationID,
			State:          execution.StateQueued,
			CreatedAt:      now,
		}
		if err := queries.CreateExecution(ctx, db.CreateExecutionParams{
			ID: created.ID, ConversationID: created.ConversationID, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("create assistant execution: %w", err)
		}
		if err := queries.CreateMessage(ctx, db.CreateMessageParams{
			ID:               uuid.NewString(),
			ConversationID:   conversationID,
			ExecutionID:      created.ID,
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
		return execution.AgentExecution{}, fmt.Errorf("create assistant execution transaction: %w", err)
	}
	return created, nil
}

// CancelExecution 立即取消排队执行；已经开始的执行只记录请求，由持有租约的实例终止模型调用。
func (s *Store) CancelExecution(ctx context.Context, actorID, executionID string) (execution.AgentExecution, error) {
	stored, err := s.queries.GetExecution(ctx, db.GetExecutionParams{ID: executionID, ActorID: actorID})
	if err != nil {
		return execution.AgentExecution{}, executionNotFound(err)
	}
	var result execution.AgentExecution
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
		result, err = executionFromDB(locked)
		if err != nil {
			return fmt.Errorf("decode assistant execution: %w", err)
		}
		switch result.State {
		case execution.StateQueued:
			now := time.Now().UTC()
			rows, err := queries.CancelQueuedExecution(ctx, db.CancelQueuedExecutionParams{
				FinishedAt: sql.NullTime{Time: now, Valid: true}, ID: executionID,
			})
			if err != nil {
				return fmt.Errorf("cancel queued assistant execution: %w", err)
			}
			if rows != 1 {
				return execution.ErrStateConflict
			}
			result.State = execution.StateCancelled
			result.FinishedAt = &now
		case execution.StateRunning:
			if result.CancellationRequested {
				return nil
			}
			rows, err := queries.RequestExecutionCancellation(ctx, executionID)
			if err != nil {
				return fmt.Errorf("request assistant execution cancellation: %w", err)
			}
			if rows != 1 {
				return execution.ErrStateConflict
			}
			result.CancellationRequested = true
		}
		return nil
	})
	if err != nil {
		return execution.AgentExecution{}, fmt.Errorf("cancel assistant execution transaction: %w", err)
	}
	return result, nil
}

// GetExecution 按所有者和执行 ID 查询一次 Agent 执行。
func (s *Store) GetExecution(ctx context.Context, actorID, id string) (execution.AgentExecution, error) {
	item, err := s.queries.GetExecution(ctx, db.GetExecutionParams{ID: id, ActorID: actorID})
	if err != nil {
		return execution.AgentExecution{}, executionNotFound(err)
	}
	result, err := executionFromDB(item)
	if err != nil {
		return execution.AgentExecution{}, fmt.Errorf("decode assistant execution: %w", err)
	}
	return result, nil
}

func executionFromDB(item db.AssistantAgentExecution) (execution.AgentExecution, error) {
	state, err := executionStateFromDB(item.State)
	if err != nil {
		return execution.AgentExecution{}, err
	}
	result := execution.AgentExecution{
		ID:                    item.ID,
		ConversationID:        item.ConversationID,
		State:                 state,
		Model:                 item.Model,
		ErrorCode:             execution.FailureCode(item.ErrorCode),
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

func executionStateFromDB(state uint8) (execution.State, error) {
	switch state {
	case executionStateQueued:
		return execution.StateQueued, nil
	case executionStateRunning:
		return execution.StateRunning, nil
	case executionStateSucceeded:
		return execution.StateSucceeded, nil
	case executionStateFailed:
		return execution.StateFailed, nil
	case executionStateCancelled:
		return execution.StateCancelled, nil
	default:
		return "", fmt.Errorf("invalid assistant execution state %d", state)
	}
}

func executionNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return execution.ErrNotFound
	}
	return err
}
