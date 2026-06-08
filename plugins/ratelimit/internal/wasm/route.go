package wasm

import (
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
)

const routeNamePrefix = "ingate-route"

func (h *httpContext) routeConfig() (config.RouteConfig, bool) {
	identity, ok := pluginwasm.CurrentRouteIdentity(routeNamePrefix)
	if !ok {
		return config.RouteConfig{}, false
	}

	for _, route := range h.plugin.config.Routes {
		if route.GatewayName == identity.GatewayName && route.RouteName == identity.RouteName && route.RuleName == identity.RuleName {
			return route, true
		}
	}
	return config.RouteConfig{}, false
}
