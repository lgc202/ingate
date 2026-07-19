package wasm

import (
	modelproxy "github.com/lgc202/ingate/plugins/aiproxy/internal/proxy"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
)

const (
	routeNamePrefix = "ingate-route"
	routeConfigMark = "ai"
)

func (h *httpContext) currentRoute() (modelproxy.Route, bool, bool) {
	identity, configID, aiRoute := pluginwasm.CurrentRouteConfigIdentity(routeNamePrefix, routeConfigMark)
	if !aiRoute {
		return modelproxy.Route{}, false, false
	}
	route, configured := h.plugin.proxy.Route(modelproxy.RouteKey{
		GatewayName: identity.GatewayName,
		RouteName:   identity.RouteName,
		RuleName:    identity.RuleName,
		ConfigID:    configID,
	})
	return route, true, configured
}
