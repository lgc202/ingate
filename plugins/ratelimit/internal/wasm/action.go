package wasm

import (
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

func proxyWasmAction(action pluginruntime.Action) types.Action {
	switch action.Kind {
	case pluginruntime.ActionRespond:
		pluginwasm.SendResponse(action.StatusCode, action.Headers, action.Body)
		return types.ActionPause
	case pluginruntime.ActionPause:
		return types.ActionPause
	default:
		return types.ActionContinue
	}
}
