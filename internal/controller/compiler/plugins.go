package compiler

import (
	pluginiprestriction "github.com/lgc202/ingate/pkg/plugin/iprestriction"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

// listenerFilterConfig 汇总一个 Listener 需要注入的内置治理插件
type listenerFilterConfig struct {
	ipRestriction *pluginiprestriction.PluginConfig
	rateLimit     *pluginratelimit.PluginConfig
}
