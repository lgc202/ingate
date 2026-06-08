// Package wasm 承载 ACL 插件的 Proxy-Wasm 适配层
package wasm

import (
	config "github.com/lgc202/ingate/pkg/plugin/acl"
	"github.com/lgc202/ingate/plugins/acl/internal/policy"
	aclruntime "github.com/lgc202/ingate/plugins/acl/internal/runtime"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

type pluginContext struct {
	types.DefaultPluginContext

	runner  *policy.Runner
	runtime *aclruntime.Runtime
}

type httpContext struct {
	types.DefaultHttpContext

	plugin *pluginContext
}

// Register 注册 Proxy-Wasm 插件上下文
func Register(runner *policy.Runner) {
	proxywasm.SetPluginContext(func(contextID uint32) types.PluginContext {
		return &pluginContext{runner: runner}
	})
}

func (p *pluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	data, err := proxywasm.GetPluginConfiguration()
	if err != nil && err != types.ErrorStatusNotFound {
		proxywasm.LogErrorf("read acl config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}

	pluginConfig, err := config.ParsePluginConfig(data)
	if err != nil {
		proxywasm.LogErrorf("parse acl config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}
	p.runtime = aclruntime.Compile(pluginConfig, p.runner)
	return types.OnPluginStartStatusOK
}

func (p *pluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	return &httpContext{plugin: p}
}
