package wasm

import (
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
)

const routeNamePrefix = "ingate-route"

type routeKey struct {
	gatewayName string
	routeName   string
}

type routeLimits struct {
	config          config.RouteConfig
	ruleName        string
	requiredHeaders []string
}

type routeIndex map[routeKey]routeLimits

func newRouteIndex(cfg config.PluginConfig) routeIndex {
	routes := make(routeIndex, len(cfg.Routes))
	for _, item := range cfg.Routes {
		routes[routeKey{gatewayName: item.GatewayName, routeName: item.RouteName}] = routeLimits{
			config:          item,
			requiredHeaders: policy.RequiredHeaders(item),
		}
	}
	return routes
}

func (i routeIndex) lookup(identity pluginwasm.RouteIdentity) (routeLimits, bool) {
	route, ok := i[routeKey{gatewayName: identity.GatewayName, routeName: identity.RouteName}]
	if !ok {
		return routeLimits{}, false
	}
	route.ruleName = identity.RuleName
	return route, true
}

func (h *httpContext) route() (routeLimits, bool) {
	identity, ok := pluginwasm.CurrentRouteIdentity(routeNamePrefix)
	if !ok {
		return routeLimits{}, false
	}
	return h.plugin.routes.lookup(identity)
}
