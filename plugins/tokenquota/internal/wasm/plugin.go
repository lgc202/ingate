// Package wasm 承载 TokenQuota 插件的 Proxy-Wasm 适配层
package wasm

import (
	config "github.com/lgc202/ingate/pkg/plugin/tokenquota"
	"github.com/lgc202/ingate/plugins/internal/redisabi"
	"github.com/lgc202/ingate/plugins/tokenquota/internal/policy"
	"github.com/lgc202/ingate/plugins/tokenquota/internal/usage"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

type pluginContext struct {
	types.DefaultPluginContext

	contextID uint32
	routes    routeIndex
}

type httpContext struct {
	types.DefaultHttpContext

	plugin            *pluginContext
	contextID         uint32
	checks            []policy.Check
	checkOutcomes     []policy.CheckOutcome
	nextCheck         int
	bookings          []policy.Check
	nextBooking       int
	bookingTokens     int64
	responseActive    bool
	responseStreaming bool
	streamUsage       usage.Stream
}

// Register 注册 Proxy-Wasm 插件上下文
func Register() {
	proxywasm.SetPluginContext(func(contextID uint32) types.PluginContext {
		return &pluginContext{contextID: contextID}
	})
}

// OnPluginStart 初始化系统 Redis 并构建 RouteRule 配额索引
func (p *pluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	if err := redisabi.Init(); err != nil {
		proxywasm.LogErrorf("initialize token quota system Redis failed: %v", err)
		return types.OnPluginStartStatusFailed
	}

	data, err := proxywasm.GetPluginConfiguration()
	if err != nil && err != types.ErrorStatusNotFound {
		proxywasm.LogErrorf("read token quota config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}
	pluginConfig, err := config.ParsePluginConfig(data)
	if err != nil {
		proxywasm.LogErrorf("parse token quota config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}
	p.routes = newRouteIndex(pluginConfig)
	return types.OnPluginStartStatusOK
}

// NewHttpContext 为每条 HTTP 请求创建独立执行上下文
func (p *pluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	redisabi.RegisterHTTPContext(p.contextID, contextID)
	return &httpContext{plugin: p, contextID: contextID}
}
