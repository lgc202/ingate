package wasm

import (
	"github.com/lgc202/ingate/plugins/acl/internal/policy"
	aclruntime "github.com/lgc202/ingate/plugins/acl/internal/runtime"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
)

func requestFromProxyWasm(route aclruntime.Route) policy.Request {
	return policy.Request{
		GatewayName: route.Config.GatewayName,
		RouteName:   route.Config.RouteName,
		RuleName:    route.Config.RuleName,
		RemoteAddr:  pluginwasm.SourceAddress(),
		Headers:     pluginwasm.RequestHeaders(route.HeaderNames),
	}
}
