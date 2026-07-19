package wasm

import (
	modelproxy "github.com/lgc202/ingate/plugins/aiproxy/internal/proxy"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

func sendLocalResponse(response modelproxy.LocalResponse) types.Action {
	pluginwasm.SendResponse(response.StatusCode, response.Headers, response.Body)
	return types.ActionPause
}
