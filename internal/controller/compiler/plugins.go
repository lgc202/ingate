package compiler

import (
	pluginacl "github.com/lgc202/ingate/pkg/plugin/acl"
	pluginaiproxy "github.com/lgc202/ingate/pkg/plugin/aiproxy"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	plugintokenquota "github.com/lgc202/ingate/pkg/plugin/tokenquota"
)

// listenerPluginConfig 汇总一个 Listener 需要注入的内置插件私有配置
type listenerPluginConfig struct {
	aiProxy       *pluginaiproxy.PluginConfig
	accessControl *pluginacl.PluginConfig
	rateLimit     *pluginratelimit.PluginConfig
	tokenQuota    *plugintokenquota.PluginConfig
}

func (c listenerPluginConfig) requiresAIUsage(gatewayName, routeName, ruleName string) bool {
	if c.tokenQuota == nil {
		return false
	}
	for _, route := range c.tokenQuota.Routes {
		if route.GatewayName == gatewayName && route.RouteName == routeName && route.RuleName == ruleName {
			return true
		}
	}
	return false
}
