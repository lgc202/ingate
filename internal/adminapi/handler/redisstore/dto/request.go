package dto

import (
	"errors"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Validate 校验创建 RedisStore 请求
func (r *CreateRedisStoreReq) Validate() error {
	return r.RedisStoreConfig.Validate()
}

// Validate 校验更新 RedisStore 请求
func (r *UpdateRedisStoreReq) Validate() error {
	if r.Version == "" {
		return errors.New("版本不能为空")
	}
	return r.RedisStoreConfig.Validate()
}

// Validate 校验 RedisStore 核心配置
func (c *RedisStoreConfig) Validate() error {
	if c.Name == "" {
		return errors.New("名称不能为空")
	}
	switch c.Mode {
	case resource.RedisModeStandalone, resource.RedisModeCluster, resource.RedisModeSentinel:
	default:
		return errors.New("Redis 模式不正确")
	}
	if c.Address == "" && len(c.Addresses) == 0 {
		return errors.New("Redis 地址不能为空")
	}
	if c.Mode == resource.RedisModeSentinel && c.SentinelMaster == "" {
		return errors.New("Redis Sentinel master 不能为空")
	}
	if c.DB < 0 {
		return errors.New("Redis DB 不能小于 0")
	}
	if c.ConnectTimeoutMillis < 0 || c.CommandTimeoutMillis < 0 {
		return errors.New("Redis 超时时间不能小于 0")
	}
	if c.PoolSize < 0 || c.MinIdleConns < 0 {
		return errors.New("Redis 连接池参数不能小于 0")
	}
	return nil
}
