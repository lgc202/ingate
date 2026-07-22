package wasm

import (
	config "github.com/lgc202/ingate/pkg/plugin/tokenquota"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/tokenquota/internal/policy"
)

const routeNamePrefix = "ingate-route"

type routeKey struct {
	gatewayName string
	routeName   string
	ruleName    string
}

type routeQuota struct {
	config          config.RouteConfig
	requiredHeaders []string
}

type routeIndex map[routeKey]routeQuota

func newRouteIndex(cfg config.PluginConfig) routeIndex {
	routes := make(routeIndex, len(cfg.Routes))
	for _, item := range cfg.Routes {
		routes[routeKey{
			gatewayName: item.GatewayName,
			routeName:   item.RouteName,
			ruleName:    item.RuleName,
		}] = routeQuota{
			config:          item,
			requiredHeaders: policy.RequiredHeaders(item),
		}
	}
	return routes
}

func (h *httpContext) route() (routeQuota, bool) {
	identity, ok := pluginwasm.CurrentRouteIdentity(routeNamePrefix)
	if !ok {
		return routeQuota{}, false
	}
	route, exists := h.plugin.routes[routeKey{
		gatewayName: identity.GatewayName,
		routeName:   identity.RouteName,
		ruleName:    identity.RuleName,
	}]
	return route, exists
}
