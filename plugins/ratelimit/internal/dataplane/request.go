package dataplane

import (
	"fmt"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
)

// NewCheckRequest 将插件内部 global checks 转换成 ingate-dataplane 协议请求
func NewCheckRequest(redisStores []config.RedisStore, checks []policy.GlobalCheck) (dataplaneratelimit.CheckRequest, error) {
	stores := make(map[string]config.RedisStore, len(redisStores))
	for _, store := range redisStores {
		stores[store.Name] = store
	}

	request := dataplaneratelimit.CheckRequest{
		Checks: make([]dataplaneratelimit.Check, 0, len(checks)),
	}
	for _, check := range checks {
		store, ok := stores[check.RedisStore]
		if !ok {
			return dataplaneratelimit.CheckRequest{}, fmt.Errorf("redis store %q not found", check.RedisStore)
		}
		request.Checks = append(request.Checks, dataplaneratelimit.Check{
			PolicyName: check.Policy.Name,
			RuleName:   check.Rule.Name,
			RedisKey:   check.RedisKey,
			RedisStore: redisStore(store),
			Algorithm:  dataplaneratelimit.Algorithm(check.Rule.Algorithm),
			Limit: dataplaneratelimit.Limit{
				Requests:      check.Requests,
				WindowSeconds: check.WindowSeconds,
				Burst:         check.Burst,
			},
			TimeoutMillis: check.RedisTimeoutMs,
		})
	}
	return request, nil
}

func redisStore(store config.RedisStore) dataplaneratelimit.RedisStore {
	return dataplaneratelimit.RedisStore{
		ID:                   store.Name,
		Mode:                 dataplaneratelimit.RedisMode(store.Mode),
		Address:              store.Address,
		Addresses:            store.Addresses,
		DB:                   store.DB,
		TLS:                  store.TLS,
		TLSServerName:        store.TLSServerName,
		Username:             store.Username,
		PasswordRef:          store.PasswordRef,
		ConnectTimeoutMillis: store.ConnectTimeoutMillis,
		CommandTimeoutMillis: store.CommandTimeoutMillis,
		PoolSize:             store.PoolSize,
		MinIdleConns:         store.MinIdleConns,
		SentinelMaster:       store.SentinelMaster,
	}
}
