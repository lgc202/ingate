package wasm

import "github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"

func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	route, ok := h.route()
	if !ok {
		return types.ActionContinue
	}

	return proxyWasmAction(h.plugin.runtime.Apply(route, requestFromProxyWasm(route)))
}
