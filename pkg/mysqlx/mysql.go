// Package mysqlx 统一 Ingate 服务连接 MySQL 的配置和初始化方式
package mysqlx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Config 定义 MySQL 连接池配置
type Config struct {
	DSN                   string        `json:"dsn" mapstructure:"dsn"`
	MaxOpenConnections    int           `json:"max_open_connections" mapstructure:"max_open_connections"`
	MaxIdleConnections    int           `json:"max_idle_connections" mapstructure:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `json:"connection_max_lifetime" mapstructure:"connection_max_lifetime"`
}

// Validate 校验 MySQL 连接池配置
func (c Config) Validate() error {
	if strings.TrimSpace(c.DSN) == "" {
		return errors.New("mysql DSN must not be empty")
	}
	if c.MaxOpenConnections <= 0 {
		return errors.New("mysql max open connections must be greater than zero")
	}
	if c.MaxIdleConnections < 0 || c.MaxIdleConnections > c.MaxOpenConnections {
		return errors.New("mysql max idle connections must be between zero and max open connections")
	}
	if c.ConnectionMaxLifetime <= 0 {
		return errors.New("mysql connection max lifetime must be greater than zero")
	}
	return nil
}

// NewDB 创建连接池并确认 MySQL 在服务启动时可用
func NewDB(ctx context.Context, config Config) (*sql.DB, error) {
	database, err := sql.Open("mysql", config.DSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	database.SetMaxOpenConns(config.MaxOpenConnections)
	database.SetMaxIdleConns(config.MaxIdleConnections)
	database.SetConnMaxLifetime(config.ConnectionMaxLifetime)
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	return database, nil
}
