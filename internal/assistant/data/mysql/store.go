// Package mysql 使用 MySQL 持久化运维助手的会话、Agent 执行和消息。
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

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
	dsnConfig := driverConfig(config)
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

func driverConfig(config *conf.Data_MySQL) mysqldriver.Config {
	return mysqldriver.Config{
		User:      config.GetUsername(),
		Passwd:    config.GetPassword(),
		Net:       "tcp",
		Addr:      config.GetAddress(),
		DBName:    config.GetDatabase(),
		ParseTime: true,
		Loc:       time.UTC,
		Timeout:   config.GetDialTimeout().AsDuration(),
		// 所有数据库生成的业务时间都以 UTC 解释，避免 Assistant 实例时钟参与状态机。
		Params: map[string]string{"time_zone": "'+00:00'"},
	}
}

// withTransaction 统一 Store 的事务提交与回滚；具体锁顺序仍由各业务操作显式表达。
func (s *Store) withTransaction(ctx context.Context, operation func(*db.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin MySQL transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := operation(s.queries.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit MySQL transaction: %w", err)
	}
	return nil
}

func mapConversationNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.ErrNotFound
	}
	return err
}
