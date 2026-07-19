package wasm

import (
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
)

func requestFromProxyWasm(route routeLimits) policy.RequestAttributes {
	return policy.RequestAttributes{
		GatewayName: route.config.GatewayName,
		RouteName:   route.config.RouteName,
		RuleName:    route.ruleName,
		Path:        pluginwasm.RequestHeader(":path"),
		RemoteAddr:  pluginwasm.SourceAddress(),
		Headers:     pluginwasm.RequestHeaders(route.requiredHeaders),
	}
}
