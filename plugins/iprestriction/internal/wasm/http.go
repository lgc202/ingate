package wasm

import (
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

const deniedMessage = "IP address is not allowed"

func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	route, ok := h.route()
	if !ok {
		return types.ActionContinue
	}
	if route.Allows(pluginwasm.SourceAddress()) {
		return types.ActionContinue
	}
	pluginwasm.SendResponse(403, nil, deniedMessage)
	return types.ActionPause
}
