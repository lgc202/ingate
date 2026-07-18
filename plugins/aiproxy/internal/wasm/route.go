package wasm

import (
	aiproxyruntime "github.com/lgc202/ingate/plugins/aiproxy/internal/runtime"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
)

const (
	routeNamePrefix = "ingate-route"
	routeConfigMark = "ai"
)

func (h *httpContext) currentRoute() (aiproxyruntime.Route, bool, bool) {
	identity, configID, aiRoute := pluginwasm.CurrentRouteConfigIdentity(routeNamePrefix, routeConfigMark)
	if !aiRoute {
		return aiproxyruntime.Route{}, false, false
	}
	route, configured := h.plugin.runtime.Route(pluginruntime.RouteKey{
		GatewayName: identity.GatewayName,
		RouteName:   identity.RouteName,
		RuleName:    identity.RuleName,
		ConfigID:    configID,
	})
	return route, true, configured
}
