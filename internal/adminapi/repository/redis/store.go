// Package redis 保存 Admin API 的登录态。
package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// Store 持有登录态使用的 Redis 连接池。
type Store struct {
	client *goredis.Client
}

// Open 建立 Redis 客户端并验证连接。
func Open(ctx context.Context, url string) (*Store, error) {
	options, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse Redis URL: %w", err)
	}

	client := goredis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		// 连接尚未交给调用方；清理失败也以原始连接错误为准。
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}

	return &Store{client: client}, nil
}

// Close 关闭 Redis 连接池。
func (s *Store) Close() error {
	return s.client.Close()
}
