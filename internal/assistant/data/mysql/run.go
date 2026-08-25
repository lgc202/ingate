package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// Run 状态在数据库中使用小整数保存，状态机语义仍由业务层的可读常量表达。
const (
	runStateRunning uint8 = iota + 1
	runStateSucceeded
	runStateFailed
)

// BeginRun 原子地创建 Run 和用户消息，并保证一个会话只有一个运行中 Run。
func (s *Store) BeginRun(
	ctx context.Context,
	actorID string,
	conversationID string,
	content string,
	model string,
) (conversation.Run, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return conversation.Run{}, fmt.Errorf("begin assistant run: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := s.queries.WithTx(tx)
	if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
		ID: conversationID, ActorID: actorID,
	}); err != nil {
		return conversation.Run{}, mapNotFound(err)
	}
	running, err := queries.CountRunningRuns(ctx, conversationID)
	if err != nil {
		return conversation.Run{}, fmt.Errorf("count running assistant runs: %w", err)
	}
	if running > 0 {
		return conversation.Run{}, conversation.ErrRunRunning
	}

	now := time.Now().UTC()
	run := conversation.Run{
		ID:             uuid.NewString(),
		ConversationID: conversationID,
		State:          conversation.StateRunning,
		Model:          model,
		StartedAt:      now,
	}
	if err := queries.CreateRun(ctx, db.CreateRunParams{
		ID:             run.ID,
		ConversationID: run.ConversationID,
		State:          runStateRunning,
		Model:          run.Model,
		StartedAt:      run.StartedAt,
	}); err != nil {
		return conversation.Run{}, fmt.Errorf("create assistant run: %w", err)
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
		return conversation.Run{}, fmt.Errorf("create user message: %w", err)
	}
	if err := queries.TouchConversation(ctx, db.TouchConversationParams{
		UpdatedAt: now,
		ID:        conversationID,
		ActorID:   actorID,
	}); err != nil {
		return conversation.Run{}, fmt.Errorf("update conversation activity: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return conversation.Run{}, fmt.Errorf("commit assistant run: %w", err)
	}
	return run, nil
}

// CompleteRun 原子地保存模型最终回复并把 Run 标记为成功。
func (s *Store) CompleteRun(
	ctx context.Context,
	actorID string,
	runID string,
	result conversation.ModelResult,
) (conversation.Message, error) {
	run, err := s.queries.GetRun(ctx, db.GetRunParams{ID: runID, ActorID: actorID})
	if err != nil {
		return conversation.Message{}, mapNotFound(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return conversation.Message{}, fmt.Errorf("begin assistant run completion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := s.queries.WithTx(tx)
	// 所有写操作统一先锁会话再锁 Run，避免发送消息和删除会话形成反向锁顺序。
	if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
		ID: run.ConversationID, ActorID: actorID,
	}); err != nil {
		return conversation.Message{}, mapNotFound(err)
	}
	run, err = queries.GetRunForUpdate(ctx, db.GetRunForUpdateParams{ID: runID, ActorID: actorID})
	if err != nil {
		return conversation.Message{}, mapNotFound(err)
	}
	if run.State != runStateRunning {
		return conversation.Message{}, conversation.ErrRunStateConflict
	}

	now := time.Now().UTC()
	message := conversation.Message{
		ID:               uuid.NewString(),
		ConversationID:   run.ConversationID,
		RunID:            run.ID,
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
		return conversation.Message{}, fmt.Errorf("create assistant message: %w", err)
	}
	rows, err := queries.CompleteRun(ctx, db.CompleteRunParams{
		FinishedAt: sql.NullTime{Time: now, Valid: true},
		ID:         runID,
	})
	if err != nil {
		return conversation.Message{}, fmt.Errorf("complete assistant run: %w", err)
	}
	if rows != 1 {
		return conversation.Message{}, conversation.ErrRunStateConflict
	}
	if err := queries.TouchConversation(ctx, db.TouchConversationParams{
		UpdatedAt: now,
		ID:        run.ConversationID,
		ActorID:   actorID,
	}); err != nil {
		return conversation.Message{}, fmt.Errorf("update conversation activity: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return conversation.Message{}, fmt.Errorf("commit assistant run completion: %w", err)
	}
	return message, nil
}

// FailRun 把运行中的 Run 标记为失败，并保存稳定错误码供接口展示和统计。
func (s *Store) FailRun(ctx context.Context, actorID, runID, errorCode string) error {
	run, err := s.queries.GetRun(ctx, db.GetRunParams{ID: runID, ActorID: actorID})
	if err != nil {
		return mapNotFound(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin assistant run failure: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := s.queries.WithTx(tx)
	// Run 的两种终态采用相同的会话、Run 锁顺序，删除会话时不会形成反向等待。
	if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
		ID: run.ConversationID, ActorID: actorID,
	}); err != nil {
		return mapNotFound(err)
	}
	run, err = queries.GetRunForUpdate(ctx, db.GetRunForUpdateParams{ID: runID, ActorID: actorID})
	if err != nil {
		return mapNotFound(err)
	}
	if run.State != runStateRunning {
		return conversation.ErrRunStateConflict
	}

	now := time.Now().UTC()
	rows, err := queries.FailRun(ctx, db.FailRunParams{
		ErrorCode:  errorCode,
		FinishedAt: sql.NullTime{Time: now, Valid: true},
		ID:         runID,
	})
	if err != nil {
		return fmt.Errorf("fail assistant run: %w", err)
	}
	if rows != 1 {
		return conversation.ErrRunStateConflict
	}
	if err := queries.TouchConversation(ctx, db.TouchConversationParams{
		UpdatedAt: now,
		ID:        run.ConversationID,
		ActorID:   actorID,
	}); err != nil {
		return fmt.Errorf("update conversation activity: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit assistant run failure: %w", err)
	}
	return nil
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
		ID:             item.ID,
		ConversationID: item.ConversationID,
		State:          state,
		Model:          item.Model,
		ErrorCode:      item.ErrorCode,
		StartedAt:      item.StartedAt,
	}
	if item.FinishedAt.Valid {
		result.FinishedAt = &item.FinishedAt.Time
	}
	return result, nil
}

func runStateFromDB(state uint8) (string, error) {
	switch state {
	case runStateRunning:
		return conversation.StateRunning, nil
	case runStateSucceeded:
		return conversation.StateSucceeded, nil
	case runStateFailed:
		return conversation.StateFailed, nil
	default:
		return "", fmt.Errorf("invalid assistant run state %d", state)
	}
}
