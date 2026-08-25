package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	modelbiz "github.com/lgc202/ingate/internal/assistant/biz/model"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// GetModelConnection 读取当前模型连接。表中无单例行表示系统尚未配置模型。
func (s *Store) GetModelConnection(ctx context.Context) (modelbiz.Connection, error) {
	item, err := s.queries.GetAssistantModelConnection(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return modelbiz.Connection{}, modelbiz.ErrNotConfigured
	}
	if err != nil {
		return modelbiz.Connection{}, fmt.Errorf("get assistant model connection: %w", err)
	}
	return modelConnectionFromDB(item), nil
}

// UpdateModelConnection 在单个事务中保留或替换 API Key，避免并发修改丢失凭据。
func (s *Store) UpdateModelConnection(
	ctx context.Context,
	update modelbiz.Update,
) (modelbiz.Connection, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return modelbiz.Connection{}, fmt.Errorf("begin assistant model connection update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := s.queries.WithTx(tx)
	apiKey := ""
	current, err := queries.GetAssistantModelConnectionForUpdate(ctx)
	switch {
	case err == nil:
		apiKey = current.ApiKey
	case !errors.Is(err, sql.ErrNoRows):
		return modelbiz.Connection{}, fmt.Errorf("lock assistant model connection: %w", err)
	}
	if update.APIKey != nil {
		apiKey = *update.APIKey
	}

	connection := update.Connection
	connection.APIKey = apiKey
	connection.Configured = true
	connection.UpdatedAt = time.Now().UTC()
	if err := queries.UpsertAssistantModelConnection(ctx, modelConnectionToDB(connection)); err != nil {
		return modelbiz.Connection{}, fmt.Errorf("upsert assistant model connection: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return modelbiz.Connection{}, fmt.Errorf("commit assistant model connection update: %w", err)
	}
	return connection, nil
}

func modelConnectionFromDB(item db.AssistantModelConnection) modelbiz.Connection {
	return modelbiz.Connection{
		Configured:            true,
		Mode:                  modelbiz.Mode(item.ConnectionMode),
		Protocol:              modelbiz.Protocol(item.Protocol),
		Endpoint:              item.Endpoint,
		APIKey:                item.ApiKey,
		Model:                 item.Model,
		Timeout:               time.Duration(item.TimeoutMs) * time.Millisecond,
		MaxOutputTokens:       int(item.MaxOutputTokens),
		ReasoningBudgetTokens: int(item.ReasoningBudgetTokens),
		UpdatedAt:             item.UpdatedAt,
	}
}

func modelConnectionToDB(connection modelbiz.Connection) db.UpsertAssistantModelConnectionParams {
	return db.UpsertAssistantModelConnectionParams{
		ConnectionMode:        uint8(connection.Mode),
		Protocol:              uint8(connection.Protocol),
		Endpoint:              connection.Endpoint,
		ApiKey:                connection.APIKey,
		Model:                 connection.Model,
		TimeoutMs:             uint32(connection.Timeout.Milliseconds()),
		MaxOutputTokens:       uint32(connection.MaxOutputTokens),
		ReasoningBudgetTokens: uint32(connection.ReasoningBudgetTokens),
		UpdatedAt:             connection.UpdatedAt,
	}
}
