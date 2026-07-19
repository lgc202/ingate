package wasm

import (
	"github.com/lgc202/ingate/plugins/acl/internal/policy"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
)

func requestFromProxyWasm(route policy.Route) policy.RequestAttributes {
	return policy.RequestAttributes{
		RemoteAddr: pluginwasm.SourceAddress(),
		Headers:    pluginwasm.RequestHeaders(route.RequiredHeaders),
	}
}
