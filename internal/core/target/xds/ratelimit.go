package xds

import pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"

// RateLimitConfig 表示 xDS target 内部保留的内置限流插件编译结果
type RateLimitConfig struct {
	Bindings    []pluginratelimit.Binding    `json:"bindings,omitempty"`
	RedisStores []pluginratelimit.RedisStore `json:"redisStores,omitempty"`
	DataPlane   *pluginratelimit.DataPlane   `json:"dataPlane,omitempty"`
}
