package wasm

import (
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
)

func requestFromProxyWasm(route config.RouteConfig) policy.Request {
	return policy.Request{
		GatewayName: route.GatewayName,
		RouteName:   route.RouteName,
		RuleName:    route.RuleName,
		Path:        pluginwasm.RequestHeader(":path"),
		RemoteAddr:  pluginwasm.SourceAddress(),
		Headers:     pluginwasm.RequestHeaders(policy.HeaderNames(route)),
	}
}

func sendRejected(decision policy.Decision) {
	pluginwasm.SendResponse(decision.StatusCode, decision.QuotaHeaders, decision.Message)
}
