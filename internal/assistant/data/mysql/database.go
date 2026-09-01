// Package mysql 提供运维助手的 MySQL 连接与可用性检查。
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/lgc202/ingate/internal/assistant/conf"
)

// Database 持有 Assistant 共享的 MySQL 连接池。
type Database struct {
	connection *sql.DB
}

// New 创建 MySQL 连接池；真实连通性由就绪检查持续确认。
func New(config *conf.Data_MySQL) (*Database, error) {
	driverConfig := mysqldriver.Config{
		User:      config.GetUsername(),
		Passwd:    config.GetPassword(),
		Net:       "tcp",
		Addr:      config.GetAddress(),
		DBName:    config.GetDatabase(),
		ParseTime: true,
		Loc:       time.UTC,
		Timeout:   config.GetDialTimeout().AsDuration(),
	}
	connection, err := sql.Open("mysql", driverConfig.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open MySQL: %w", err)
	}
	connection.SetMaxOpenConns(int(config.GetMaxOpenConnections()))
	connection.SetMaxIdleConns(int(config.GetMaxIdleConnections()))
	connection.SetConnMaxLifetime(config.GetConnectionMaxLifetime().AsDuration())
	return &Database{connection: connection}, nil
}

// Check 确认 MySQL 能在调用方期限内响应。
func (d *Database) Check(ctx context.Context) error {
	return d.connection.PingContext(ctx)
}

// Close 释放 MySQL 连接池。
func (d *Database) Close() error {
	return d.connection.Close()
}
