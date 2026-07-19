package compiler

import (
	pluginacl "github.com/lgc202/ingate/pkg/plugin/acl"
	pluginaiproxy "github.com/lgc202/ingate/pkg/plugin/aiproxy"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

// listenerPluginConfig 汇总一个 Listener 需要注入的内置插件私有配置
type listenerPluginConfig struct {
	aiProxy       *pluginaiproxy.PluginConfig
	accessControl *pluginacl.PluginConfig
	rateLimit     *pluginratelimit.PluginConfig
}
