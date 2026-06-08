package wasm

import (
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	ratelimitruntime "github.com/lgc202/ingate/plugins/ratelimit/internal/runtime"
)

const routeNamePrefix = "ingate-route"

func (h *httpContext) route() (ratelimitruntime.Route, bool) {
	identity, ok := pluginwasm.CurrentRouteIdentity(routeNamePrefix)
	if !ok {
		return ratelimitruntime.Route{}, false
	}
	if h.plugin.runtime == nil {
		return ratelimitruntime.Route{}, false
	}
	return h.plugin.runtime.Route(pluginruntime.RouteKey{
		GatewayName: identity.GatewayName,
		RouteName:   identity.RouteName,
		RuleName:    identity.RuleName,
	})
}
