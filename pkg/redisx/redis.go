// Package redisx 提供 Ingate 组件共用的 Redis 客户端连接能力
package redisx

import (
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lgc202/ingate/pkg/tlsx"
)

// Config 定义 Redis 单节点连接和连接池参数
type Config struct {
	Address            string
	Username           string
	Password           string
	Database           int
	DialTimeout        time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	PoolSize           int
	MinIdleConnections int
	TLS                tlsx.ClientConfig
}

// NewClient 创建 Redis 客户端连接池
func NewClient(config Config) (*redis.Client, error) {
	if config.Address == "" {
		return nil, errors.New("redis address must not be empty")
	}
	if config.Database < 0 {
		return nil, errors.New("redis database must not be negative")
	}
	if config.DialTimeout <= 0 || config.ReadTimeout <= 0 || config.WriteTimeout <= 0 {
		return nil, errors.New("redis timeouts must be greater than zero")
	}
	if config.PoolSize <= 0 || config.MinIdleConnections < 0 || config.MinIdleConnections > config.PoolSize {
		return nil, errors.New("redis connection pool limits are invalid")
	}
	tlsConfig, err := tlsx.NewClient(config.TLS)
	if err != nil {
		return nil, fmt.Errorf("configure Redis TLS: %w", err)
	}
	return redis.NewClient(&redis.Options{
		Addr:         config.Address,
		Username:     config.Username,
		Password:     config.Password,
		DB:           config.Database,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConnections,
		TLSConfig:    tlsConfig,
	}), nil
}
