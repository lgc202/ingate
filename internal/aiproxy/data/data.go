// Package data 管理 AI Proxy 使用的外部数据连接
package data

import (
	"context"
	"time"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"

	"github.com/lgc202/ingate/internal/aiproxy/conf"
	"github.com/lgc202/ingate/pkg/redisx"
)

const connectTimeout = 10 * time.Second

// ProviderSet 汇总 AI Proxy 的数据连接
var ProviderSet = wire.NewSet(NewData, NewRedisClient)

// Data 持有 AI Proxy 使用的外部数据连接
type Data struct {
	rdb *redis.Client
}

// NewData 创建外部数据连接，清理函数由 Kratos App 生命周期统一调用
func NewData(config *conf.Data) (*Data, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	rdb, err := redisx.NewClient(ctx, redisConfig(config.GetRedis()))
	if err != nil {
		return nil, nil, err
	}
	data := &Data{rdb: rdb}
	cleanup := func() {
		// 进程退出后的关闭错误不可恢复，连接最终由操作系统回收
		_ = data.rdb.Close()
	}
	return data, cleanup, nil
}

// NewRedisClient 提供访问凭据认证和就绪检查使用的 Redis 连接
func NewRedisClient(data *Data) *redis.Client {
	return data.rdb
}

func redisConfig(config *conf.Data_Redis) redisx.Config {
	return redisx.Config{
		Address:  config.GetAddress(),
		Password: config.GetPassword(),
		Database: int(config.GetDatabase()),
	}
}
