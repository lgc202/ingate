package ratelimit

import (
	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	"github.com/lgc202/ingate/pkg/xredis"
)

func redisConfig(store dataplaneratelimit.RedisStore) xredis.Config {
	return xredis.Config{
		ID:                   store.ID,
		Mode:                 xredis.Mode(store.Mode),
		Address:              store.Address,
		Addresses:            store.Addresses,
		DB:                   store.DB,
		TLS:                  store.TLS,
		TLSServerName:        store.TLSServerName,
		Username:             store.Username,
		Password:             store.Password,
		ConnectTimeoutMillis: store.ConnectTimeoutMillis,
		CommandTimeoutMillis: store.CommandTimeoutMillis,
		PoolSize:             store.PoolSize,
		MinIdleConns:         store.MinIdleConns,
		SentinelMaster:       store.SentinelMaster,
	}
}
