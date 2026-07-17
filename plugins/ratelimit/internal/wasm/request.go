package wasm

import (
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
	ratelimitruntime "github.com/lgc202/ingate/plugins/ratelimit/internal/runtime"
)

func requestFromProxyWasm(route ratelimitruntime.Route) policy.Request {
	return policy.Request{
		GatewayName: route.Config.GatewayName,
		RouteName:   route.Config.RouteName,
		RuleName:    route.RuleName,
		Path:        pluginwasm.RequestHeader(":path"),
		RemoteAddr:  pluginwasm.SourceAddress(),
		Headers:     pluginwasm.RequestHeaders(route.HeaderNames),
	}
}
