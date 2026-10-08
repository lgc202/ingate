// Package redis 保存 Admin API 的登录态。
package redis

import goredis "github.com/redis/go-redis/v9"

// Store 使用组件持有的 Redis 客户端读写登录态。
type Store struct {
	client goredis.UniversalClient
}

// New 使用已有 Redis 客户端创建 Store；客户端由组件负责关闭。
func New(client goredis.UniversalClient) *Store {
	return &Store{client: client}
}
