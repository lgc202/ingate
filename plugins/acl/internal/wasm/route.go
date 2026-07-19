package wasm

import (
	"github.com/lgc202/ingate/plugins/acl/internal/policy"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
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
