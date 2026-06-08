package dto

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// RedisStoreConfig 是控制台读写 RedisStore 时复用的核心配置
type RedisStoreConfig struct {
	Name                 string             `json:"name"`
	Description          string             `json:"description,omitempty"`
	Mode                 resource.RedisMode `json:"mode"`
	Address              string             `json:"address"`
	Addresses            []string           `json:"addresses,omitempty"`
	DB                   int                `json:"db,omitempty"`
	TLS                  bool               `json:"tls"`
	TLSServerName        string             `json:"tlsServerName,omitempty"`
	Username             string             `json:"username,omitempty"`
	PasswordRef          string             `json:"passwordRef,omitempty"`
	ConnectTimeoutMillis int                `json:"connectTimeoutMillis,omitempty"`
	CommandTimeoutMillis int                `json:"commandTimeoutMillis,omitempty"`
	PoolSize             int                `json:"poolSize,omitempty"`
	MinIdleConns         int                `json:"minIdleConns,omitempty"`
	SentinelMaster       string             `json:"sentinelMaster,omitempty"`
}

// CreateRedisStoreReq 是创建 RedisStore 的请求体
type CreateRedisStoreReq struct {
	RedisStoreConfig
}

// UpdateRedisStoreReq 是更新 RedisStore 的请求体
type UpdateRedisStoreReq struct {
	Version string `json:"version"`
	RedisStoreConfig
}

// RedisStore 是 admin-api 面向控制台返回的 Redis 配置
type RedisStore struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
	RedisStoreConfig
	CreatedAt string `json:"createdAt"`
}

// ListRedisStoresResp 是 RedisStore 列表响应
type ListRedisStoresResp struct {
	RedisStores []RedisStore `json:"redisStores"`
}

// CreateRedisStoreResp 是创建 RedisStore 响应
type CreateRedisStoreResp struct {
	Success bool   `json:"success"`
	ID      string `json:"id,omitempty"`
}

// UpdateRedisStoreResp 是更新 RedisStore 响应
type UpdateRedisStoreResp struct {
	Success bool `json:"success"`
}

// DeleteRedisStoreResp 是删除 RedisStore 响应
type DeleteRedisStoreResp struct {
	Success bool `json:"success"`
}
