// Package wasm 承载 AI Proxy 插件的 Proxy-Wasm 适配层
package wasm

import (
	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
	modelproxy "github.com/lgc202/ingate/plugins/aiproxy/internal/proxy"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

type pluginContext struct {
	types.DefaultPluginContext

	proxy *modelproxy.Proxy
}

type httpContext struct {
	types.DefaultHttpContext

	plugin            *pluginContext
	route             modelproxy.Route
	responseTransform *modelproxy.ResponseTransform
	responseStream    *modelproxy.ResponseStream
	responseStatus    int
	requestActive     bool
	responseBuffered  bool
	responseStreaming bool
	responseClosed    bool
	streamFailed      bool
}

// Register 注册 Proxy-Wasm 插件上下文
func Register() {
	proxywasm.SetPluginContext(func(contextID uint32) types.PluginContext {
		return &pluginContext{}
	})
}

// OnPluginStart 解析配置并构建当前 Listener 的模型路由索引
func (p *pluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	data, err := proxywasm.GetPluginConfiguration()
	if err != nil && err != types.ErrorStatusNotFound {
		proxywasm.LogErrorf("read AI proxy config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}

	pluginConfig, err := config.ParsePluginConfig(data)
	if err != nil {
		proxywasm.LogErrorf("parse AI proxy config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}
	p.proxy = modelproxy.New(pluginConfig)
	return types.OnPluginStartStatusOK
}

// NewHttpContext 为每条 HTTP 请求创建独立执行上下文
func (p *pluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	return &httpContext{plugin: p}
}
