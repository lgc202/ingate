package compiler

import (
	pluginiprestriction "github.com/lgc202/ingate/pkg/plugin/iprestriction"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	plugintokenquota "github.com/lgc202/ingate/pkg/plugin/tokenquota"
)

// listenerFilterConfig 汇总一个 Listener 需要注入的治理插件和 AI ExtProc 配置
type listenerFilterConfig struct {
	aiProxy       bool
	ipRestriction *pluginiprestriction.PluginConfig
	rateLimit     *pluginratelimit.PluginConfig
	tokenQuota    *plugintokenquota.PluginConfig
}

func (c listenerFilterConfig) requiresAIUsage(gatewayName, routeName, ruleName string) bool {
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
