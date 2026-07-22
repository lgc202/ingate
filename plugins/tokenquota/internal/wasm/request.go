package wasm

import (
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/lgc202/ingate/plugins/tokenquota/internal/policy"
)

func requestFromProxyWasm(route routeQuota) policy.RequestAttributes {
	return policy.RequestAttributes{
		RemoteAddr: pluginwasm.SourceAddress(),
		Headers:    pluginwasm.RequestHeaders(route.requiredHeaders),
	}
}
