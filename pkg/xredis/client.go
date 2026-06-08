package xredis

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Manager 按 Redis 配置复用连接池
type Manager struct {
	mu      sync.Mutex
	clients map[string]cachedClient
}

type cachedClient struct {
	fingerprint string
	client      redis.UniversalClient
}

// NewManager 创建 Redis client 管理器
func NewManager() *Manager {
	return &Manager{clients: map[string]cachedClient{}}
}

// NewClient 根据配置创建 Redis client，调用方负责关闭返回的 client
func NewClient(config Config) (redis.UniversalClient, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	options := &redis.UniversalOptions{
		Addrs:        redisAddresses(config),
		DB:           config.DB,
		Username:     config.Username,
		Password:     config.Password,
		DialTimeout:  config.DialTimeout(),
		ReadTimeout:  config.CommandTimeout(),
		WriteTimeout: config.CommandTimeout(),
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MasterName:   config.SentinelMaster,
	}
	if config.TLS {
		options.TLSConfig = &tls.Config{
			ServerName: config.TLSServerName,
			MinVersion: tls.VersionTLS12,
		}
	}
	return redis.NewUniversalClient(options), nil
}

// Client 返回配置对应的 Redis client，配置变化时会关闭旧连接并替换
func (m *Manager) Client(config Config) (redis.UniversalClient, error) {
	fingerprint, err := configFingerprint(config)
	if err != nil {
		return nil, err
	}

	key := config.ID
	if key == "" {
		key = fingerprint
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if cached, ok := m.clients[key]; ok && cached.fingerprint == fingerprint {
		return cached.client, nil
	}

	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}
	if cached, ok := m.clients[key]; ok {
		_ = cached.client.Close()
	}
	m.clients[key] = cachedClient{
		fingerprint: fingerprint,
		client:      client,
	}
	return client, nil
}

// Close 关闭 Manager 持有的全部 Redis client
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var err error
	for key, cached := range m.clients {
		if closeErr := cached.client.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		delete(m.clients, key)
	}
	return err
}

func validateConfig(config Config) error {
	if len(redisAddresses(config)) == 0 {
		return errors.New("redis address is required")
	}
	switch config.Mode {
	case "", ModeStandalone, ModeCluster, ModeSentinel:
	default:
		return errors.New("unsupported redis mode")
	}
	if config.Mode == ModeSentinel && config.SentinelMaster == "" {
		return errors.New("redis sentinel master is required")
	}
	if config.DB < 0 {
		return errors.New("redis db must be greater than or equal to zero")
	}
	if config.ConnectTimeoutMillis < 0 || config.CommandTimeoutMillis < 0 {
		return errors.New("redis timeout must be greater than or equal to zero")
	}
	if config.PoolSize < 0 || config.MinIdleConns < 0 {
		return errors.New("redis pool size must be greater than or equal to zero")
	}
	return nil
}

func configFingerprint(config Config) (string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("marshal redis config fingerprint: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func redisAddresses(config Config) []string {
	if len(config.Addresses) > 0 {
		return config.Addresses
	}
	if config.Address != "" {
		return []string{config.Address}
	}
	return nil
}

func durationMillis(value int) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Millisecond
}
