package xredis

import "time"

// Mode 表示 Redis 部署模式
type Mode string

const (
	// ModeStandalone 表示单实例 Redis
	ModeStandalone Mode = "Standalone"
	// ModeSentinel 表示 Redis Sentinel
	ModeSentinel Mode = "Sentinel"
	// ModeCluster 表示 Redis Cluster
	ModeCluster Mode = "Cluster"
)

// Config 表示创建 Redis client 所需的连接配置
type Config struct {
	ID                   string
	Mode                 Mode
	Address              string
	Addresses            []string
	DB                   int
	TLS                  bool
	TLSServerName        string
	Username             string
	Password             string
	ConnectTimeoutMillis int
	CommandTimeoutMillis int
	PoolSize             int
	MinIdleConns         int
	SentinelMaster       string
}

// DialTimeout 返回 Redis 建连超时
func (c Config) DialTimeout() time.Duration {
	return durationMillis(c.ConnectTimeoutMillis)
}

// CommandTimeout 返回 Redis 命令读写超时
func (c Config) CommandTimeout() time.Duration {
	return durationMillis(c.CommandTimeoutMillis)
}
