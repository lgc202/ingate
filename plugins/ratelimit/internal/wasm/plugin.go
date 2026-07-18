// Package wasm 承载 RateLimit 插件的 Proxy-Wasm 适配层
package wasm

import (
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"github.com/lgc202/ingate/plugins/internal/redisabi"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/redis"
	ratelimitruntime "github.com/lgc202/ingate/plugins/ratelimit/internal/runtime"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

type pluginContext struct {
	types.DefaultPluginContext

	contextID uint32
	runtime   *ratelimitruntime.Runtime
}

type httpContext struct {
	types.DefaultHttpContext

	plugin       *pluginContext
	contextID    uint32
	quotaHeaders map[string]string
	checks       []policy.Check
	outcomes     []policy.Outcome
	requests     []redis.Request
	index        int
}

// Register 注册 Proxy-Wasm 插件上下文
func Register() {
	proxywasm.SetPluginContext(func(contextID uint32) types.PluginContext {
		return &pluginContext{contextID: contextID}
	})
}

func (p *pluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	if err := redisabi.Init(); err != nil {
		proxywasm.LogErrorf("initialize rate-limit system Redis failed: %v", err)
		return types.OnPluginStartStatusFailed
	}

	data, err := proxywasm.GetPluginConfiguration()
	if err != nil && err != types.ErrorStatusNotFound {
		proxywasm.LogErrorf("read rate-limit config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}

	pluginConfig, err := config.ParsePluginConfig(data)
	if err != nil {
		proxywasm.LogErrorf("parse rate-limit config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}
	p.runtime = ratelimitruntime.Compile(pluginConfig)
	return types.OnPluginStartStatusOK
}

func (p *pluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	redisabi.RegisterHTTPContext(p.contextID, contextID)
	return &httpContext{plugin: p, contextID: contextID}
}
