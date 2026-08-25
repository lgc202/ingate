// Package mysql 使用 MySQL 持久化运维助手的会话、Run 和消息。
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// Store 将 sqlc 查询组合为业务层需要的持久化操作和事务边界。
type Store struct {
	db      *sql.DB
	queries *db.Queries
}

// NewStore 创建连接池，并在返回前确认 MySQL 可用。
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
	connection, err := sql.Open("mysql", dsnConfig.FormatDSN())
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

// Close 释放 Store 持有的 MySQL 连接池。
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping 检查 MySQL 是否能够在当前请求期限内响应。
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.ErrNotFound
	}
	return err
}
