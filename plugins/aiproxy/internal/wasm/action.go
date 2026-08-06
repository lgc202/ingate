package wasm

import (
	modelproxy "github.com/lgc202/ingate/plugins/aiproxy/internal/proxy"
	"github.com/lgc202/ingate/plugins/internal/redisabi"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

func sendLocalResponse(response modelproxy.LocalResponse) types.Action {
	pluginwasm.SendResponse(response.StatusCode, response.Headers, response.Body)
	return types.ActionPause
}

func sendPausedResponse(contextID uint32, response modelproxy.LocalResponse) error {
	return redisabi.SendHTTPResponse(contextID, response.StatusCode, response.Headers, response.Body)
}
