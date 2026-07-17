package wasm

import (
	"github.com/lgc202/ingate/plugins/acl/internal/policy"
	aclruntime "github.com/lgc202/ingate/plugins/acl/internal/runtime"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
)

func requestFromProxyWasm(route aclruntime.Route) policy.Request {
	return policy.Request{
		RemoteAddr: pluginwasm.SourceAddress(),
		Headers:    pluginwasm.RequestHeaders(route.HeaderNames),
	}
}
