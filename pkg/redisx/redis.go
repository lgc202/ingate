// Package redisx 统一 Ingate 服务连接 Redis 的基础配置和初始化方式
package redisx

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

// Config 定义 Redis 连接配置
type Config struct {
	Address  string `json:"address" mapstructure:"address"`
	Password string `json:"password,omitempty" mapstructure:"password"`
	Database int    `json:"database" mapstructure:"database"`
}

// Validate 校验 Redis 连接配置
func (c Config) Validate() error {
	if strings.TrimSpace(c.Address) == "" {
		return errors.New("redis address must not be empty")
	}
	if c.Database < 0 {
		return errors.New("redis database must not be negative")
	}
	return nil
}

// NewClient 创建客户端并确认 Redis 在服务启动时可用
func NewClient(ctx context.Context, config Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB:       config.Database,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return client, nil
}
