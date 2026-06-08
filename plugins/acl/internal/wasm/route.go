package wasm

import (
	aclruntime "github.com/lgc202/ingate/plugins/acl/internal/runtime"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
)

const routeNamePrefix = "ingate-route"

func (h *httpContext) route() (aclruntime.Route, bool) {
	identity, ok := pluginwasm.CurrentRouteIdentity(routeNamePrefix)
	if !ok {
		return aclruntime.Route{}, false
	}
	if h.plugin.runtime == nil {
		return aclruntime.Route{}, false
	}
	return h.plugin.runtime.Route(pluginruntime.RouteKey{
		GatewayName: identity.GatewayName,
		RouteName:   identity.RouteName,
		RuleName:    identity.RuleName,
	})
}
