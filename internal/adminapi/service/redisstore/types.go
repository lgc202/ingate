package redisstore

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 表示 RedisStore 列表查询结果
type ListResult struct {
	RedisStores []resource.RedisStore
}

// RedisStoreResult 表示单个 RedisStore 查询结果
type RedisStoreResult struct {
	RedisStore *resource.RedisStore
}

// RedisStoreParams 表示 RedisStore 可编辑字段
type RedisStoreParams struct {
	Name                 string
	Description          string
	Mode                 resource.RedisMode
	Address              string
	Addresses            []string
	DB                   int
	TLS                  bool
	TLSServerName        string
	Username             string
	PasswordRef          string
	ConnectTimeoutMillis int
	CommandTimeoutMillis int
	PoolSize             int
	MinIdleConns         int
	SentinelMaster       string
}

// CreateRedisStoreParams 表示创建 RedisStore 参数
type CreateRedisStoreParams struct {
	RedisStoreParams
}

// UpdateRedisStoreParams 表示更新 RedisStore 参数
type UpdateRedisStoreParams struct {
	Version string
	RedisStoreParams
}
