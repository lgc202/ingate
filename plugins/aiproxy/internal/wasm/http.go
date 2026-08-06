package wasm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/bearer"
	auth "github.com/lgc202/ingate/plugins/aiproxy/internal/accesskey"
	modelproxy "github.com/lgc202/ingate/plugins/aiproxy/internal/proxy"
	"github.com/lgc202/ingate/plugins/internal/redisabi"
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

// OnHttpRequestHeaders 清理不可信请求头并通过系统 Redis 认证客户端访问密钥
func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	method := pluginwasm.RequestHeader(":method")
	path := pluginwasm.RequestHeader(":path")
	authorization := pluginwasm.RequestHeader(authorizationHeader)
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
		return sendLocalResponse(h.plugin.proxy.InternalError())
	}
	if !configured {
		proxywasm.LogError("AI proxy route configuration is missing or stale")
		return sendLocalResponse(h.plugin.proxy.InternalError())
	}

	h.route = route
	if response := h.plugin.proxy.ValidateEndpoint(method, path); response != nil {
		return sendLocalResponse(*response)
	}
	secret, ok := bearerSecret(authorization)
	if !ok {
		return sendLocalResponse(h.plugin.proxy.Unauthorized())
	}
	h.authenticationSecret = secret
	h.requestActive = true
	if endOfStream {
		if err := h.dispatchAuthentication(); err != nil {
			proxywasm.LogErrorf("dispatch AI access key authentication failed: %v", err)
			return sendLocalResponse(h.plugin.proxy.AuthenticationUnavailable())
		}
	}
	// 认证和模型选择都完成前不向上游发送请求头
	return types.ActionPause
}

// OnHttpRequestBody 缓冲并转换请求体，然后写入目标 Cluster、路径和认证信息
func (h *httpContext) OnHttpRequestBody(bodySize int, endOfStream bool) types.Action {
	if h.authenticationPending {
		return types.ActionPause
	}
	if !h.requestActive {
		return types.ActionContinue
	}
	if bodySize > modelproxy.MaxRequestBodyBytes {
		h.requestActive = false
		return sendLocalResponse(h.plugin.proxy.RequestTooLarge())
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
			return sendLocalResponse(h.plugin.proxy.InternalError())
		}
	}
	h.requestBody = body
	if err := h.dispatchAuthentication(); err != nil {
		proxywasm.LogErrorf("dispatch AI access key authentication failed: %v", err)
		h.requestActive = false
		return sendLocalResponse(h.plugin.proxy.AuthenticationUnavailable())
	}
	return types.ActionPause
}

// OnHttpStreamDone 关闭当前 HTTP context 的 Redis callback 生命周期
func (h *httpContext) OnHttpStreamDone() {
	redisabi.CloseHTTPContext(h.plugin.contextID, h.contextID)
}

func (h *httpContext) handleAuthentication(result redisabi.Result) {
	h.authenticationPending = false
	if result.Err != nil || result.Status != redisabi.RedisStatusOK {
		err := result.Err
		if err == nil {
			err = fmt.Errorf("redis call failed with status %d", result.Status)
		}
		proxywasm.LogErrorf("complete AI access key authentication failed: %v", err)
		if err := sendPausedResponse(h.contextID, h.plugin.proxy.AuthenticationUnavailable()); err != nil {
			proxywasm.LogErrorf("send AI access key unavailable response failed: %v", err)
		}
		return
	}
	grant, authorized, err := auth.Decode(result.Data)
	if err != nil {
		proxywasm.LogErrorf("decode AI access key authentication failed: %v", err)
		if err := sendPausedResponse(h.contextID, h.plugin.proxy.AuthenticationUnavailable()); err != nil {
			proxywasm.LogErrorf("send AI access key decode failure response failed: %v", err)
		}
		return
	}
	if !authorized {
		if err := sendPausedResponse(h.contextID, h.plugin.proxy.Unauthorized()); err != nil {
			proxywasm.LogErrorf("send AI access key unauthorized response failed: %v", err)
		}
		return
	}

	request, response := h.plugin.proxy.PrepareRequest(h.route, h.requestBody)
	if response != nil {
		if err := sendPausedResponse(h.contextID, *response); err != nil {
			proxywasm.LogErrorf("send invalid AI request response failed: %v", err)
		}
		return
	}
	if !grant.Allows(request.Response.PublicModel) {
		if err := sendPausedResponse(h.contextID, h.plugin.proxy.ModelForbidden(request.Response.PublicModel)); err != nil {
			proxywasm.LogErrorf("send AI access key model permission response failed: %v", err)
		}
		return
	}
	if err := h.applyPreparedRequest(request); err != nil {
		proxywasm.LogErrorf("apply AI proxy request failed: %v", err)
		if sendErr := sendPausedResponse(h.contextID, h.plugin.proxy.InternalError()); sendErr != nil {
			proxywasm.LogErrorf("send AI proxy request failure response failed: %v", sendErr)
		}
		return
	}
	h.authenticationSecret = ""
	h.requestBody = nil
	h.requestActive = false
	if err := redisabi.ResumeHTTPRequest(h.contextID); err != nil {
		proxywasm.LogErrorf("resume authenticated AI request failed: %v", err)
	}
}

func (h *httpContext) dispatchAuthentication() error {
	command, err := auth.Command(h.authenticationSecret)
	if err != nil {
		return fmt.Errorf("encode Redis command: %w", err)
	}
	h.authenticationPending = true
	if _, err := redisabi.Dispatch(h.plugin.contextID, h.contextID, command, h.handleAuthentication); err != nil {
		h.authenticationPending = false
		return err
	}
	return nil
}

func (h *httpContext) applyPreparedRequest(request modelproxy.PreparedRequest) error {
	if err := proxywasm.ReplaceHttpRequestBody(request.Body); err != nil {
		return fmt.Errorf("replace request body: %w", err)
	}
	for _, header := range [][2]string{
		{":path", request.Path},
		{aiClusterHeader, request.Cluster},
		{aiRouteHeader, request.RouteConfigID},
		{contentTypeHeader, jsonContentType},
		{contentLengthHeader, strconv.Itoa(len(request.Body))},
	} {
		if err := replaceRequestHeader(header[0], header[1]); err != nil {
			return err
		}
	}
	for _, header := range request.Headers {
		if err := replaceRequestHeader(header.Name, header.Value); err != nil {
			return err
		}
	}
	h.responseTransform = &request.Response
	return nil
}

func bearerSecret(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || !bearer.ValidToken(parts[1]) {
		return "", false
	}
	return parts[1], true
}

// OnHttpResponseHeaders 选择普通响应缓冲或 SSE 增量转换模式
func (h *httpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	if h.responseTransform == nil {
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
		return h.sendJSONResponse(502, h.plugin.proxy.ResponseError())
	}

	if h.responseTransform.Stream && status < 400 {
		stream, err := h.plugin.proxy.NewResponseStream(*h.responseTransform)
		if err != nil {
			proxywasm.LogErrorf("create AI proxy response stream failed: %v", err)
			return h.sendJSONResponse(502, h.plugin.proxy.ResponseError())
		}
		if err := replaceResponseHeader(contentTypeHeader, sseContentType); err != nil {
			proxywasm.LogErrorf("set AI proxy SSE content type failed: %v", err)
			return h.sendJSONResponse(502, h.plugin.proxy.ResponseError())
		}
		h.responseStream = stream
		h.responseStreaming = true
		return types.ActionContinue
	}

	if err := replaceResponseHeader(contentTypeHeader, jsonContentType); err != nil {
		proxywasm.LogErrorf("set AI proxy JSON content type failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.proxy.ResponseError())
	}
	h.responseBuffered = true
	if !endOfStream {
		return types.ActionPause
	}

	// endOfStream 表示不会再进入 body 回调，需要在响应头仍未下发时直接重建规范响应
	transformed, err := h.plugin.proxy.TransformResponse(*h.responseTransform, h.responseStatus, nil)
	if err != nil {
		proxywasm.LogErrorf("transform empty AI proxy response failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.proxy.ResponseError())
	}
	return h.sendJSONResponse(h.responseStatus, transformed)
}

// OnHttpResponseBody 转换普通响应或任意边界的 SSE 分块
func (h *httpContext) OnHttpResponseBody(bodySize int, endOfStream bool) types.Action {
	if h.responseTransform == nil {
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
	if bodySize > modelproxy.MaxResponseBodyBytes {
		proxywasm.LogErrorf("AI proxy response body exceeds %d bytes", modelproxy.MaxResponseBodyBytes)
		return h.sendJSONResponse(502, h.plugin.proxy.ResponseError())
	}
	if !endOfStream {
		return types.ActionPause
	}
	body, err := proxywasm.GetHttpResponseBody(0, bodySize)
	if err != nil {
		proxywasm.LogErrorf("read AI proxy response body failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.proxy.ResponseError())
	}
	transformed, err := h.plugin.proxy.TransformResponse(*h.responseTransform, h.responseStatus, body)
	if err != nil {
		proxywasm.LogErrorf("transform AI proxy response failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.proxy.ResponseError())
	}
	if err := proxywasm.ReplaceHttpResponseBody(transformed); err != nil {
		proxywasm.LogErrorf("replace AI proxy response body failed: %v", err)
		return h.sendJSONResponse(502, h.plugin.proxy.ResponseError())
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
	if err := proxywasm.ReplaceHttpResponseBody(h.plugin.proxy.StreamError()); err != nil {
		proxywasm.LogErrorf("replace AI proxy stream with error failed: %v", err)
		return types.ActionPause
	}
	return types.ActionContinue
}
