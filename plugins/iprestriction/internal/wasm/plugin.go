// Package wasm 承载客户端 IP 访问限制插件的 Proxy-Wasm 适配层
package wasm

import (
	config "github.com/lgc202/ingate/pkg/plugin/iprestriction"
	"github.com/lgc202/ingate/plugins/iprestriction/internal/policy"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

type pluginContext struct {
	types.DefaultPluginContext

	routes policy.Routes
}

type httpContext struct {
	types.DefaultHttpContext

	plugin *pluginContext
}

// Register 注册 Proxy-Wasm 插件上下文
func Register() {
	proxywasm.SetPluginContext(func(contextID uint32) types.PluginContext {
		return &pluginContext{}
	})
}

func (p *pluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	data, err := proxywasm.GetPluginConfiguration()
	if err != nil && err != types.ErrorStatusNotFound {
		proxywasm.LogErrorf("read IP restriction config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}

	pluginConfig, err := config.ParsePluginConfig(data)
	if err != nil {
		proxywasm.LogErrorf("parse IP restriction config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}
	p.routes, err = policy.NewRoutes(pluginConfig)
	if err != nil {
		proxywasm.LogErrorf("build IP restriction routes failed: %v", err)
		return types.OnPluginStartStatusFailed
	}
	return types.OnPluginStartStatusOK
}

func (p *pluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	return &httpContext{plugin: p}
}
