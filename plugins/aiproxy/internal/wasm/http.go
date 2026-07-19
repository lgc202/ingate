package wasm

import (
	"strconv"

	aiproxyruntime "github.com/lgc202/ingate/plugins/aiproxy/internal/runtime"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	pluginwasm "github.com/lgc202/ingate/plugins/internal/wasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

const (
	authorizationHeader    = "authorization"
	contentLengthHeader    = "content-length"
	contentEncodingHeader  = "content-encoding"
	contentTypeHeader      = "content-type"
	acceptEncodingHeader   = "accept-encoding"
	aiClusterHeader        = "x-ingate-ai-cluster-v1"
	aiRouteHeader          = "x-ingate-ai-route-v1"
	anthropicAPIKeyHeader  = "x-api-key"
	anthropicVersionHeader = "anthropic-version"
	geminiAPIKeyHeader     = "x-goog-api-key"
	jsonContentType        = "application/json"
	sseContentType         = "text/event-stream"
)

// OnHttpRequestHeaders 清理不可信请求头并暂停 AI 请求，等待请求体完成模型选择
func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	method := pluginwasm.RequestHeader(":method")
	path := pluginwasm.RequestHeader(":path")
	route, aiRoute, configured := h.currentRoute()

	if !aiRoute {
		return types.ActionContinue
	}
	// AI Route 不信任客户端提供的内部选路信息或模型厂商凭据
	if err := removeRequestHeaders(
		authorizationHeader,
		anthropicAPIKeyHeader,
		anthropicVersionHeader,
		geminiAPIKeyHeader,
		aiClusterHeader,
		aiRouteHeader,
		contentLengthHeader,
		contentEncodingHeader,
		contentTypeHeader,
		acceptEncodingHeader,
	); err != nil {
		proxywasm.LogErrorf("sanitize AI proxy request headers failed: %v", err)
		return proxyWasmAction(h.plugin.runtime.InternalError().Action)
	}
	if !configured {
		proxywasm.LogError("AI proxy route configuration is missing or stale")
		return proxyWasmAction(h.plugin.runtime.InternalError().Action)
	}

	h.route = route
	h.requestActive = true
	result := h.plugin.runtime.ValidateEndpoint(method, path)
	if result.Action.Kind == pluginruntime.ActionRespond {
		h.requestActive = false
		return proxyWasmAction(result.Action)
	}
	if endOfStream {
		h.requestActive = false
		return proxyWasmAction(h.plugin.runtime.Apply(route, nil).Action)
	}
	// 暂停 header 发送，等待完整请求体确定目标 Cluster 和厂商协议
	return types.ActionPause
}

// OnHttpRequestBody 缓冲并转换请求体，然后写入目标 Cluster、路径和认证信息
func (h *httpContext) OnHttpRequestBody(bodySize int, endOfStream bool) types.Action {
	if !h.requestActive {
		return types.ActionContinue
	}
	if bodySize > aiproxyruntime.MaxRequestBodyBytes {
		h.requestActive = false
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
			h.requestActive = false
			return proxyWasmAction(h.plugin.runtime.InternalError().Action)
		}
	}
	result := h.plugin.runtime.Apply(h.route, body)
	if result.Action.Kind == pluginruntime.ActionRespond {
		h.requestActive = false
		return proxyWasmAction(result.Action)
	}
	if err := proxywasm.ReplaceHttpRequestBody(result.Mutation.Body); err != nil {
		proxywasm.LogErrorf("replace AI proxy request body failed: %v", err)
		h.requestActive = false
		return proxyWasmAction(h.plugin.runtime.InternalError().Action)
	}
	for _, header := range [][2]string{
		{":path", result.Mutation.Path},
		{aiClusterHeader, result.Mutation.Cluster},
		{aiRouteHeader, result.Mutation.RouteConfigID},
		{contentTypeHeader, jsonContentType},
		{contentLengthHeader, strconv.Itoa(len(result.Mutation.Body))},
	} {
		if err := replaceRequestHeader(header[0], header[1]); err != nil {
			proxywasm.LogErrorf("set AI proxy routing header failed: %v", err)
			h.requestActive = false
			return proxyWasmAction(h.plugin.runtime.InternalError().Action)
		}
	}
	for _, header := range result.Mutation.Headers {
		if err := replaceRequestHeader(header.Name, header.Value); err != nil {
			proxywasm.LogErrorf("set AI proxy upstream header failed: %v", err)
			h.requestActive = false
			return proxyWasmAction(h.plugin.runtime.InternalError().Action)
		}
	}
	h.responsePlan = result.ResponsePlan
	h.requestActive = false
	return types.ActionContinue
}

// OnHttpResponseHeaders 选择普通响应缓冲或 SSE 增量转换模式
func (h *httpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	if h.responsePlan == nil {
		return types.ActionContinue
	}
	status, err := strconv.Atoi(pluginwasm.ResponseHeader(":status"))
	if err != nil {
		proxywasm.LogErrorf("parse AI proxy response status failed: %v", err)
		status = 502
	}
	h.responseStatus = status
	if err := removeResponseHeaders(contentLengthHeader, contentEncodingHeader, contentTypeHeader); err != nil {
		proxywasm.LogErrorf("sanitize AI proxy response headers failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.runtime.ResponseError())
	}

	if h.responsePlan.Stream && status < 400 {
		stream, err := h.plugin.runtime.NewResponseStream(*h.responsePlan)
		if err != nil {
			proxywasm.LogErrorf("create AI proxy response stream failed: %v", err)
			return h.sendJSONResponse(502, h.plugin.runtime.ResponseError())
		}
		if err := replaceResponseHeader(contentTypeHeader, sseContentType); err != nil {
			proxywasm.LogErrorf("set AI proxy SSE content type failed: %v", err)
			return h.sendJSONResponse(502, h.plugin.runtime.ResponseError())
		}
		h.responseStream = stream
		h.responseStreaming = true
		return types.ActionContinue
	}

	if err := replaceResponseHeader(contentTypeHeader, jsonContentType); err != nil {
		proxywasm.LogErrorf("set AI proxy JSON content type failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.runtime.ResponseError())
	}
	h.responseBuffered = true
	if !endOfStream {
		return types.ActionPause
	}

	// endOfStream 表示不会再进入 body 回调，需要在响应头仍未下发时直接重建规范响应
	transformed, err := h.plugin.runtime.TransformResponse(*h.responsePlan, h.responseStatus, nil)
	if err != nil {
		proxywasm.LogErrorf("transform empty AI proxy response failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.runtime.ResponseError())
	}
	return h.sendJSONResponse(h.responseStatus, transformed)
}

// OnHttpResponseBody 转换普通响应或任意边界的 SSE 分块
func (h *httpContext) OnHttpResponseBody(bodySize int, endOfStream bool) types.Action {
	if h.responsePlan == nil {
		return types.ActionContinue
	}
	if h.responseClosed {
		return types.ActionPause
	}
	if h.responseStreaming {
		return h.transformStreamingResponse(bodySize, endOfStream)
	}
	if !h.responseBuffered {
		return types.ActionContinue
	}
	if bodySize > aiproxyruntime.MaxResponseBodyBytes {
		proxywasm.LogErrorf("AI proxy response body exceeds %d bytes", aiproxyruntime.MaxResponseBodyBytes)
		return h.sendJSONResponse(502, h.plugin.runtime.ResponseError())
	}
	if !endOfStream {
		return types.ActionPause
	}
	body, err := proxywasm.GetHttpResponseBody(0, bodySize)
	if err != nil {
		proxywasm.LogErrorf("read AI proxy response body failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.runtime.ResponseError())
	}
	transformed, err := h.plugin.runtime.TransformResponse(*h.responsePlan, h.responseStatus, body)
	if err != nil {
		proxywasm.LogErrorf("transform AI proxy response failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.runtime.ResponseError())
	}
	if err := proxywasm.ReplaceHttpResponseBody(transformed); err != nil {
		proxywasm.LogErrorf("replace AI proxy response body failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.runtime.ResponseError())
	}
	return types.ActionContinue
}

// sendJSONResponse 在原响应头仍处于暂停状态时重建完整下游响应
//
// Proxy-Wasm 不允许在 body 回调中直接修改响应头，SendHttpResponse 是同时替换状态码、
// Header 和响应体的标准方式
func (h *httpContext) sendJSONResponse(statusCode int, body []byte) types.Action {
	h.responseClosed = true
	err := proxywasm.SendHttpResponse(
		uint32(statusCode),
		[][2]string{{contentTypeHeader, jsonContentType}},
		body,
		-1,
	)
	if err != nil {
		proxywasm.LogErrorf("send AI proxy JSON response failed: %v", err)
	}
	return types.ActionPause
}

func (h *httpContext) transformStreamingResponse(bodySize int, endOfStream bool) types.Action {
	if h.streamFailed {
		if err := proxywasm.ReplaceHttpResponseBody(nil); err != nil {
			proxywasm.LogErrorf("drop AI proxy stream tail failed: %v", err)
			return types.ActionPause
		}
		return types.ActionContinue
	}
	var body []byte
	if bodySize > 0 {
		var err error
		body, err = proxywasm.GetHttpResponseBody(0, bodySize)
		if err != nil {
			proxywasm.LogErrorf("read AI proxy stream body failed: %v", err)
			return h.failStreamingResponse()
		}
	}
	transformed, err := h.responseStream.Push(body)
	if err == nil && endOfStream {
		var tail []byte
		tail, err = h.responseStream.Finish()
		transformed = append(transformed, tail...)
	}
	if err != nil {
		proxywasm.LogErrorf("transform AI proxy stream failed: %v", err)
		return h.failStreamingResponse()
	}
	if err := proxywasm.ReplaceHttpResponseBody(transformed); err != nil {
		proxywasm.LogErrorf("replace AI proxy stream body failed: %v", err)
		return h.failStreamingResponse()
	}
	return types.ActionContinue
}

func (h *httpContext) failStreamingResponse() types.Action {
	h.streamFailed = true
	if err := proxywasm.ReplaceHttpResponseBody(h.plugin.runtime.StreamError()); err != nil {
		proxywasm.LogErrorf("replace AI proxy stream with error failed: %v", err)
		return types.ActionPause
	}
	return types.ActionContinue
}
