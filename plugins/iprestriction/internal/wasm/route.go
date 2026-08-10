package wasm

import (
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/iprestriction/internal/policy"
)

const routeNamePrefix = "ingate-route"

func (h *httpContext) route() (policy.Route, bool) {
	identity, ok := pluginwasm.CurrentRouteIdentity(routeNamePrefix)
	if !ok {
		return policy.Route{}, false
	}
	route, exists := h.plugin.routes[policy.RouteKey{
		GatewayName: identity.GatewayName,
		RouteName:   identity.RouteName,
	}]
	return route, exists
}
