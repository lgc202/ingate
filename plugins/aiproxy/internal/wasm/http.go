package wasm

import (
	aiproxyruntime "github.com/lgc202/ingate/plugins/aiproxy/internal/runtime"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

const authorizationHeader = "authorization"

// OnHttpRequestHeaders 清理不可信请求头并暂停 AI 请求，等待请求体完成模型选择
func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	method := pluginwasm.RequestHeader(":method")
	path := pluginwasm.RequestHeader(":path")
	route, aiRoute, configured := h.currentRoute()

	if !aiRoute {
		return types.ActionContinue
	}
	// AI Route 不信任客户端凭据，未配置上游 API Key 时也不会透传 Authorization
	if err := removeRequestHeaders(authorizationHeader, "content-length"); err != nil {
		proxywasm.LogErrorf("sanitize AI proxy request headers failed: %v", err)
		return proxyWasmAction(h.plugin.runtime.InternalError().Action)
	}
	if !configured {
		proxywasm.LogError("AI proxy route configuration is missing or stale")
		return proxyWasmAction(h.plugin.runtime.InternalError().Action)
	}

	h.route = route
	h.active = true
	result := h.plugin.runtime.ValidateEndpoint(method, path)
	if result.Action.Kind == pluginruntime.ActionRespond {
		h.active = false
		return proxyWasmAction(result.Action)
	}
	if endOfStream {
		h.active = false
		return proxyWasmAction(h.plugin.runtime.Apply(route, nil).Action)
	}
	// 暂停 header 发送，等待完整请求体确定实际上游模型名称
	return types.ActionPause
}

// OnHttpRequestBody 缓冲并改写 OpenAI 请求体，然后注入当前 Route 的上游凭据
func (h *httpContext) OnHttpRequestBody(bodySize int, endOfStream bool) types.Action {
	if !h.active {
		return types.ActionContinue
	}
	if bodySize > aiproxyruntime.MaxRequestBodyBytes {
		h.active = false
		return proxyWasmAction(h.plugin.runtime.RequestTooLarge().Action)
	}
	if !endOfStream {
		return types.ActionPause
	}

	var body []byte
	if bodySize > 0 {
		var err error
		body, err = proxywasm.GetHttpRequestBody(0, bodySize)
		if err != nil {
			proxywasm.LogErrorf("read AI proxy request body failed: %v", err)
			h.active = false
			return proxyWasmAction(h.plugin.runtime.InternalError().Action)
		}
	}
	result := h.plugin.runtime.Apply(h.route, body)
	if result.Action.Kind == pluginruntime.ActionRespond {
		h.active = false
		return proxyWasmAction(result.Action)
	}
	if err := proxywasm.ReplaceHttpRequestBody(result.Mutation.Body); err != nil {
		proxywasm.LogErrorf("replace AI proxy request body failed: %v", err)
		h.active = false
		return proxyWasmAction(h.plugin.runtime.InternalError().Action)
	}
	if h.route.Config.APIKey != "" {
		if err := addRequestHeader(authorizationHeader, "Bearer "+h.route.Config.APIKey); err != nil {
			proxywasm.LogErrorf("set AI proxy authorization failed: %v", err)
			h.active = false
			return proxyWasmAction(h.plugin.runtime.InternalError().Action)
		}
	}
	h.active = false
	return types.ActionContinue
}
