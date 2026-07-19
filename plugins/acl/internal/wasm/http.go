package wasm

import (
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	route, ok := h.route()
	if !ok {
		return types.ActionContinue
	}

	decision := route.Evaluate(requestFromProxyWasm(route))
	if decision.Allowed {
		return types.ActionContinue
	}
	pluginwasm.SendResponse(decision.StatusCode, nil, decision.Message)
	return types.ActionPause
}
