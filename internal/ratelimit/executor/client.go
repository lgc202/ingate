package executor

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lgc202/ingate/internal/ratelimit/protocol"
)

// ClientManager 按 RedisStore 配置复用 Redis 连接池
type ClientManager struct {
	mu      sync.Mutex
	clients map[string]redis.UniversalClient
}

// NewClientManager 创建 Redis client 管理器
func NewClientManager() *ClientManager {
	return &ClientManager{clients: map[string]redis.UniversalClient{}}
}

// Client 返回 RedisStore 对应的 Redis client
func (m *ClientManager) Client(store protocol.RedisStore) (redis.UniversalClient, error) {
	key, err := storeFingerprint(store)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[key]; ok {
		return client, nil
	}

	client, err := newRedisClient(store)
	if err != nil {
		return nil, err
	}
	m.clients[key] = client
	return client, nil
}

func storeFingerprint(store protocol.RedisStore) (string, error) {
	data, err := json.Marshal(store)
	if err != nil {
		return "", fmt.Errorf("marshal redis store fingerprint: %w", err)
	}
	return string(data), nil
}

func newRedisClient(store protocol.RedisStore) (redis.UniversalClient, error) {
	addrs := redisAddresses(store)
	if len(addrs) == 0 {
		return nil, errors.New("redis address is required")
	}

	options := &redis.UniversalOptions{
		Addrs:        addrs,
		DB:           store.DB,
		Username:     store.Username,
		Password:     store.Password,
		DialTimeout:  durationMillis(store.ConnectTimeoutMillis),
		ReadTimeout:  durationMillis(store.CommandTimeoutMillis),
		WriteTimeout: durationMillis(store.CommandTimeoutMillis),
		PoolSize:     store.PoolSize,
		MinIdleConns: store.MinIdleConns,
		MasterName:   store.SentinelMaster,
	}
	if store.TLS {
		options.TLSConfig = &tls.Config{
			ServerName: store.TLSServerName,
			MinVersion: tls.VersionTLS12,
		}
	}

	switch store.Mode {
	case "", protocol.RedisModeStandalone, protocol.RedisModeCluster, protocol.RedisModeSentinel:
		return redis.NewUniversalClient(options), nil
	default:
		return nil, errors.New("unsupported redis mode")
	}
}

func redisAddresses(store protocol.RedisStore) []string {
	if len(store.Addresses) > 0 {
		return store.Addresses
	}
	if store.Address != "" {
		return []string{store.Address}
	}
	return nil
}

func durationMillis(value int) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Millisecond
}
