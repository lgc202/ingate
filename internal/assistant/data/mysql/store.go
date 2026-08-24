// Package mysql 使用 MySQL 持久化运维助手的会话、消息和执行状态。
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// Store 将 sqlc 生成的查询组合成业务层需要的事务操作。
type Store struct {
	db      *sql.DB
	queries *db.Queries
}

// NewStore 创建连接池并验证 MySQL 可用性。
func NewStore(ctx context.Context, config *conf.Data_MySQL) (*Store, error) {
	dsnConfig := mysql.Config{
		User:      config.GetUsername(),
		Passwd:    config.GetPassword(),
		Net:       "tcp",
		Addr:      config.GetAddress(),
		DBName:    config.GetDatabase(),
		ParseTime: true,
		Loc:       time.UTC,
		Timeout:   config.GetDialTimeout().AsDuration(),
	}
	dsn := dsnConfig.FormatDSN()
	connection, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
	}
	connection.SetMaxOpenConns(int(config.GetMaxOpenConnections()))
	connection.SetMaxIdleConns(int(config.GetMaxIdleConnections()))
	connection.SetConnMaxLifetime(config.GetConnectionMaxLifetime().AsDuration())
	if err := connection.PingContext(ctx); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("connect MySQL: %w", err)
	}
	return &Store{db: connection, queries: db.New(connection)}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) Create(ctx context.Context, item conversation.Conversation) (conversation.Conversation, error) {
	err := s.queries.CreateConversation(ctx, db.CreateConversationParams{
		ID: item.ID, ActorID: item.ActorID, Title: item.Title,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	})
	if err != nil {
		return conversation.Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return item, nil
}

func (s *Store) Get(ctx context.Context, actorID, id string) (conversation.Conversation, error) {
	item, err := s.queries.GetConversation(ctx, db.GetConversationParams{ID: id, ActorID: actorID})
	if err != nil {
		return conversation.Conversation{}, mapNotFound(err)
	}
	return conversationFromDB(item), nil
}

func (s *Store) List(
	ctx context.Context,
	actorID string,
	limit int,
	cursor *conversation.ConversationCursor,
) (conversation.ConversationPage, error) {
	updatedAt := time.Date(9999, 12, 31, 23, 59, 59, 999999000, time.UTC)
	id := "~"
	if cursor != nil {
		updatedAt = cursor.UpdatedAt
		id = cursor.ID
	}
	rows, err := s.queries.ListConversations(ctx, db.ListConversationsParams{
		ActorID: actorID, UpdatedAt: updatedAt, UpdatedAt_2: updatedAt, ID: id, Limit: int32(limit + 1),
	})
	if err != nil {
		return conversation.ConversationPage{}, fmt.Errorf("list conversations: %w", err)
	}
	page := conversation.ConversationPage{Items: make([]conversation.Conversation, 0, min(len(rows), limit))}
	for _, row := range rows[:min(len(rows), limit)] {
		page.Items = append(page.Items, conversationFromDB(row))
	}
	if len(rows) > limit {
		last := page.Items[len(page.Items)-1]
		page.NextCursor = &conversation.ConversationCursor{UpdatedAt: last.UpdatedAt, ID: last.ID}
	}
	return page, nil
}

func (s *Store) Delete(ctx context.Context, actorID, id string, version int64) error {
	if _, err := s.Get(ctx, actorID, id); err != nil {
		return err
	}
	rows, err := s.queries.DeleteConversation(ctx, db.DeleteConversationParams{
		ID: id, ActorID: actorID, Version: uint64(version),
	})
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	if rows == 0 {
		return conversation.ErrVersionConflict
	}
	return nil
}

func (s *Store) ListMessages(
	ctx context.Context,
	actorID string,
	conversationID string,
	afterSequence int64,
	limit int,
) (conversation.MessagePage, error) {
	if _, err := s.Get(ctx, actorID, conversationID); err != nil {
		return conversation.MessagePage{}, err
	}
	rows, err := s.queries.ListMessages(ctx, db.ListMessagesParams{
		ConversationID: conversationID, Sequence: uint64(afterSequence), Limit: int32(limit + 1),
	})
	if err != nil {
		return conversation.MessagePage{}, fmt.Errorf("list messages: %w", err)
	}
	page := conversation.MessagePage{Items: make([]conversation.Message, 0, min(len(rows), limit))}
	for _, row := range rows[:min(len(rows), limit)] {
		page.Items = append(page.Items, messageFromDB(row))
	}
	if len(rows) > limit {
		page.NextSequence = page.Items[len(page.Items)-1].Sequence
	}
	return page, nil
}

func (s *Store) BeginExecution(
	ctx context.Context,
	actorID string,
	conversationID string,
	content string,
	model string,
) (conversation.Execution, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return conversation.Execution{}, fmt.Errorf("begin execution transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := s.queries.WithTx(tx)
	if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
		ID: conversationID, ActorID: actorID,
	}); err != nil {
		return conversation.Execution{}, mapNotFound(err)
	}
	running, err := queries.CountRunningExecutions(ctx, conversationID)
	if err != nil {
		return conversation.Execution{}, fmt.Errorf("count running executions: %w", err)
	}
	if running > 0 {
		return conversation.Execution{}, conversation.ErrExecutionRunning
	}
	now := time.Now().UTC()
	sequence, err := allocateSequence(ctx, queries, actorID, conversationID, now)
	if err != nil {
		return conversation.Execution{}, err
	}
	messageID := uuid.NewString()
	if err := queries.CreateMessage(ctx, db.CreateMessageParams{
		ID: messageID, ConversationID: conversationID, Sequence: uint64(sequence),
		Role: conversation.RoleUser, Content: content, CreatedAt: now,
	}); err != nil {
		return conversation.Execution{}, fmt.Errorf("create user message: %w", err)
	}
	execution := conversation.Execution{
		ID: uuid.NewString(), ConversationID: conversationID, State: conversation.StateRunning,
		Model: model, StartedAt: now,
	}
	if err := queries.CreateExecution(ctx, db.CreateExecutionParams{
		ID: execution.ID, ConversationID: conversationID, UserMessageID: messageID,
		State: execution.State, Model: model, StartedAt: now,
	}); err != nil {
		return conversation.Execution{}, fmt.Errorf("create execution: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return conversation.Execution{}, fmt.Errorf("commit execution transaction: %w", err)
	}
	return execution, nil
}

func (s *Store) CompleteExecution(
	ctx context.Context,
	actorID string,
	executionID string,
	content string,
) (conversation.Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return conversation.Message{}, fmt.Errorf("begin completion transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := s.queries.WithTx(tx)
	execution, err := queries.GetExecutionForUpdate(ctx, db.GetExecutionForUpdateParams{
		ID: executionID, ActorID: actorID,
	})
	if err != nil {
		return conversation.Message{}, mapNotFound(err)
	}
	if execution.State != conversation.StateRunning {
		return conversation.Message{}, conversation.ErrVersionConflict
	}
	if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
		ID: execution.ConversationID, ActorID: actorID,
	}); err != nil {
		return conversation.Message{}, mapNotFound(err)
	}
	now := time.Now().UTC()
	sequence, err := allocateSequence(ctx, queries, actorID, execution.ConversationID, now)
	if err != nil {
		return conversation.Message{}, err
	}
	message := conversation.Message{
		ID: uuid.NewString(), ConversationID: execution.ConversationID, Sequence: sequence,
		Role: conversation.RoleAssistant, Content: content, CreatedAt: now,
	}
	if err := queries.CreateMessage(ctx, db.CreateMessageParams{
		ID: message.ID, ConversationID: message.ConversationID, Sequence: uint64(message.Sequence),
		Role: message.Role, Content: message.Content, CreatedAt: message.CreatedAt,
	}); err != nil {
		return conversation.Message{}, fmt.Errorf("create assistant message: %w", err)
	}
	rows, err := queries.CompleteExecution(ctx, db.CompleteExecutionParams{
		AssistantMessageID: sql.NullString{String: message.ID, Valid: true},
		FinishedAt:         sql.NullTime{Time: now, Valid: true}, ID: executionID,
	})
	if err != nil {
		return conversation.Message{}, fmt.Errorf("complete execution: %w", err)
	}
	if rows == 0 {
		return conversation.Message{}, conversation.ErrVersionConflict
	}
	if err := tx.Commit(); err != nil {
		return conversation.Message{}, fmt.Errorf("commit completion transaction: %w", err)
	}
	return message, nil
}

func (s *Store) FailExecution(ctx context.Context, executionID, failureCode string) error {
	rows, err := s.queries.FailExecution(ctx, db.FailExecutionParams{
		FailureCode: failureCode,
		FinishedAt:  sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ID:          executionID,
	})
	if err != nil {
		return fmt.Errorf("fail execution: %w", err)
	}
	if rows == 0 {
		return conversation.ErrVersionConflict
	}
	return nil
}

func (s *Store) GetExecution(ctx context.Context, actorID, id string) (conversation.Execution, error) {
	item, err := s.queries.GetExecution(ctx, db.GetExecutionParams{ID: id, ActorID: actorID})
	if err != nil {
		return conversation.Execution{}, mapNotFound(err)
	}
	return executionFromDB(item), nil
}

func allocateSequence(
	ctx context.Context,
	queries *db.Queries,
	actorID string,
	conversationID string,
	now time.Time,
) (int64, error) {
	result, err := queries.AllocateMessageSequence(ctx, db.AllocateMessageSequenceParams{
		UpdatedAt: now, ID: conversationID, ActorID: actorID,
	})
	if err != nil {
		return 0, fmt.Errorf("allocate message sequence: %w", err)
	}
	next, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read allocated message sequence: %w", err)
	}
	// next_message_sequence 保存下一次可用值，因此本次消息使用原值 next-1。
	return next - 1, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.ErrNotFound
	}
	return err
}

func conversationFromDB(item db.AssistantConversation) conversation.Conversation {
	return conversation.Conversation{
		ID: item.ID, ActorID: item.ActorID, Title: item.Title, Version: int64(item.Version),
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func messageFromDB(item db.AssistantMessage) conversation.Message {
	return conversation.Message{
		ID: item.ID, ConversationID: item.ConversationID, Sequence: int64(item.Sequence),
		Role: item.Role, Content: item.Content, CreatedAt: item.CreatedAt,
	}
}

func executionFromDB(item db.AssistantExecution) conversation.Execution {
	result := conversation.Execution{
		ID: item.ID, ConversationID: item.ConversationID, State: item.State,
		Model: item.Model, FailureCode: item.FailureCode, StartedAt: item.StartedAt,
	}
	if item.FinishedAt.Valid {
		result.FinishedAt = &item.FinishedAt.Time
	}
	return result
}
