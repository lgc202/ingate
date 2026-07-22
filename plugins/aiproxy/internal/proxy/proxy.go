// Package proxy 处理模型路由选择、协议转换和请求响应改写
package proxy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/anthropic"
	"github.com/lgc202/ingate/pkg/llm/gemini"
	"github.com/lgc202/ingate/pkg/llm/openai"
	"github.com/lgc202/ingate/pkg/llm/sse"
	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
)

const (
	// MaxRequestBodyBytes 限制第一版文本请求的缓冲大小，避免为共享 Listener 扩大内存边界
	MaxRequestBodyBytes = 1 << 20
	// MaxResponseBodyBytes 限制需要整体转换的普通模型响应，SSE 不整体缓冲但会限制单个未完成事件
	MaxResponseBodyBytes = config.MaxResponseBodyBytes
	chatCompletionsPath  = "/v1/chat/completions"
	jsonContentType      = "application/json"
)

type rejection struct {
	statusCode int
	message    string
	typ        string
	param      string
	code       string
	allow      string
}

// Proxy 保存模型路由索引并处理 AI 请求和响应转换
type Proxy struct {
	routes map[RouteKey]Route
}

// RouteKey 标识一条需要执行 AI 模型路由的 RouteRule 配置
type RouteKey struct {
	GatewayName string
	RouteName   string
	RuleName    string
	ConfigID    string
}

// Route 表示单条 RouteRule 的预编译模型和 Upstream 索引
type Route struct {
	ConfigID     string
	RequireUsage bool
	Upstreams    map[string]config.UpstreamConfig
	Models       map[string]config.ModelConfig
}

// PreparedRequest 保存发送给模型 Upstream 的完整请求改写结果
type PreparedRequest struct {
	Body          []byte
	Path          string
	Cluster       string
	RouteConfigID string
	Headers       []config.HeaderConfig
	Response      ResponseTransform
}

// ResponseTransform 保存当前请求的响应协议转换信息
type ResponseTransform struct {
	Protocol    llm.Protocol
	PublicModel string
	Stream      bool
}

// LocalResponse 表示 AI Proxy 在请求进入模型 Upstream 前直接返回的响应
type LocalResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

type responseStream interface {
	Push([]byte) ([]byte, error)
	Finish() ([]byte, error)
}

// ResponseStream 统一封装当前请求对应的上游 SSE 转换器
type ResponseStream struct {
	stream responseStream
}

// New 根据插件配置构建模型代理
func New(cfg config.PluginConfig) *Proxy {
	routes := make(map[RouteKey]Route, len(cfg.Routes))
	for _, routeConfig := range cfg.Routes {
		upstreams := make(map[string]config.UpstreamConfig, len(routeConfig.Upstreams))
		for _, upstream := range routeConfig.Upstreams {
			upstreams[upstream.ID] = upstream
		}
		models := make(map[string]config.ModelConfig, len(routeConfig.Models))
		for _, model := range routeConfig.Models {
			models[model.Model] = model
		}
		routes[RouteKey{
			GatewayName: routeConfig.GatewayName,
			RouteName:   routeConfig.RouteName,
			RuleName:    routeConfig.RuleName,
			ConfigID:    routeConfig.ConfigID,
		}] = Route{
			ConfigID:     routeConfig.ConfigID,
			RequireUsage: routeConfig.RequireUsage,
			Upstreams:    upstreams,
			Models:       models,
		}
	}
	return &Proxy{routes: routes}
}

// Route 返回命中当前 xDS route identity 的 AI Proxy 执行配置
func (p *Proxy) Route(key RouteKey) (Route, bool) {
	route, ok := p.routes[key]
	return route, ok
}

// ValidateEndpoint 校验请求是否属于第一版支持的 OpenAI API
func (p *Proxy) ValidateEndpoint(method, path string) *LocalResponse {
	if method != "POST" {
		response := rejectionResponse(rejection{
			statusCode: 405,
			message:    "Only POST is supported for this endpoint",
			typ:        "invalid_request_error",
			code:       "method_not_allowed",
			allow:      "POST",
		})
		return &response
	}
	requestPath, _, _ := strings.Cut(path, "?")
	if requestPath != chatCompletionsPath {
		response := rejectionResponse(rejection{
			statusCode: 404,
			message:    "The requested endpoint is not supported",
			typ:        "invalid_request_error",
			code:       "endpoint_not_found",
		})
		return &response
	}
	return nil
}

// PrepareRequest 根据请求体选择模型 Upstream，并生成路径、认证和协议转换结果
func (p *Proxy) PrepareRequest(route Route, body []byte) (PreparedRequest, *LocalResponse) {
	request, err := openai.DecodeRequest(body)
	if err != nil {
		code := "invalid_request"
		if errors.Is(err, llm.ErrUnsupportedFeature) {
			code = "unsupported_feature"
		}
		response := rejectionResponse(rejection{
			statusCode: 400,
			message:    err.Error(),
			typ:        "invalid_request_error",
			code:       code,
		})
		return PreparedRequest{}, &response
	}
	model, exists := route.Models[request.Model]
	if !exists {
		response := rejectionResponse(rejection{
			statusCode: 404,
			message:    fmt.Sprintf("The model %q does not exist or is not available for this route", request.Model),
			typ:        "invalid_request_error",
			param:      "model",
			code:       "model_not_found",
		})
		return PreparedRequest{}, &response
	}
	upstream, exists := route.Upstreams[model.UpstreamID]
	if !exists {
		response := p.InternalError()
		return PreparedRequest{}, &response
	}

	transformedBody, upstreamPath, err := transformRequest(upstream, model.UpstreamModel, request, route.RequireUsage)
	if err != nil {
		if errors.Is(err, llm.ErrInvalidRequest) || errors.Is(err, llm.ErrUnsupportedFeature) {
			response := rejectionResponse(rejection{
				statusCode: 400,
				message:    err.Error(),
				typ:        "invalid_request_error",
				code:       "unsupported_request",
			})
			return PreparedRequest{}, &response
		}
		response := p.InternalError()
		return PreparedRequest{}, &response
	}
	headers := make([]config.HeaderConfig, 0, len(upstream.Headers)+1)
	headers = append(headers, upstream.Headers...)
	if upstream.APIKey != "" {
		headers = append(headers, config.HeaderConfig{
			Name:  upstream.APIKeyHeader,
			Value: upstream.APIKeyPrefix + upstream.APIKey,
		})
	}
	return PreparedRequest{
		Body:          transformedBody,
		Path:          upstreamPath,
		Cluster:       upstream.Cluster,
		RouteConfigID: route.ConfigID,
		Headers:       headers,
		Response: ResponseTransform{
			Protocol:    upstream.Protocol,
			PublicModel: model.Model,
			Stream:      request.Streaming(),
		},
	}, nil
}

// TransformResponse 把普通响应或上游错误统一转换为 OpenAI-compatible JSON
func (p *Proxy) TransformResponse(transform ResponseTransform, statusCode int, body []byte) ([]byte, error) {
	if statusCode >= 400 {
		switch transform.Protocol {
		case llm.ProtocolOpenAIChatCompletions:
			return openai.TransformError(body, statusCode), nil
		case llm.ProtocolAnthropicMessages:
			return anthropic.TransformError(body, statusCode), nil
		case llm.ProtocolGeminiGenerateContent:
			return gemini.TransformError(body, statusCode), nil
		default:
			return nil, fmt.Errorf("unsupported response protocol %q", transform.Protocol)
		}
	}
	switch transform.Protocol {
	case llm.ProtocolOpenAIChatCompletions:
		return openai.TransformResponse(body, transform.PublicModel)
	case llm.ProtocolAnthropicMessages:
		return anthropic.TransformResponse(body, transform.PublicModel)
	case llm.ProtocolGeminiGenerateContent:
		return gemini.TransformResponse(body, transform.PublicModel)
	default:
		return nil, fmt.Errorf("unsupported response protocol %q", transform.Protocol)
	}
}

// NewResponseStream 创建当前请求对应的 SSE 转换器
func (p *Proxy) NewResponseStream(transform ResponseTransform) (*ResponseStream, error) {
	var (
		stream responseStream
		err    error
	)
	switch transform.Protocol {
	case llm.ProtocolOpenAIChatCompletions:
		stream, err = openai.NewStream(transform.PublicModel)
	case llm.ProtocolAnthropicMessages:
		stream, err = anthropic.NewStream(transform.PublicModel)
	case llm.ProtocolGeminiGenerateContent:
		stream, err = gemini.NewStream(transform.PublicModel)
	default:
		err = fmt.Errorf("unsupported response protocol %q", transform.Protocol)
	}
	if err != nil {
		return nil, err
	}
	return &ResponseStream{stream: stream}, nil
}

// Push 转换一个任意边界的 SSE 网络分块
func (s *ResponseStream) Push(chunk []byte) ([]byte, error) {
	return s.stream.Push(chunk)
}

// Finish 完成 SSE 转换并校验上游结束状态
func (s *ResponseStream) Finish() ([]byte, error) {
	return s.stream.Finish()
}

// StreamError 把流转换异常编码成客户端仍可识别的 SSE 错误和结束标记
func (p *Proxy) StreamError() []byte {
	detail := openai.DefaultError(502, "The upstream stream could not be processed")
	body := sse.EncodeData(openai.EncodeError(detail))
	return append(body, sse.EncodeData([]byte("[DONE]"))...)
}

// ResponseError 返回普通上游响应无法转换时的 OpenAI-compatible 错误
func (p *Proxy) ResponseError() []byte {
	return openai.EncodeError(openai.DefaultError(502, "The upstream response could not be processed"))
}

// RequestTooLarge 返回请求体超过插件缓冲上限时的本地响应
func (p *Proxy) RequestTooLarge() LocalResponse {
	return rejectionResponse(rejection{
		statusCode: 413,
		message:    "Request body is too large",
		typ:        "invalid_request_error",
		code:       "request_too_large",
	})
}

// InternalError 返回插件无法完成请求改写时的本地响应
func (p *Proxy) InternalError() LocalResponse {
	return rejectionResponse(rejection{
		statusCode: 500,
		message:    "The request could not be processed",
		typ:        "server_error",
		code:       "internal_error",
	})
}

func transformRequest(
	upstream config.UpstreamConfig,
	upstreamModel string,
	request openai.Request,
	requireUsage bool,
) ([]byte, string, error) {
	var (
		transformed []byte
		endpoint    string
		err         error
	)
	switch upstream.Protocol {
	case llm.ProtocolOpenAIChatCompletions:
		if requireUsage && request.Streaming() {
			transformed, err = openai.TransformRequestWithStreamUsage(request, upstreamModel)
		} else {
			transformed, err = openai.TransformRequest(request, upstreamModel)
		}
		endpoint = openai.ChatCompletionsPath
	case llm.ProtocolAnthropicMessages:
		transformed, err = anthropic.TransformRequest(request, upstreamModel)
		endpoint = anthropic.MessagesPath
	case llm.ProtocolGeminiGenerateContent:
		transformed, err = gemini.TransformRequest(request)
		if err == nil {
			endpoint, err = gemini.EndpointPath(upstreamModel, request.Streaming())
		}
	default:
		err = fmt.Errorf("unsupported request protocol %q", upstream.Protocol)
	}
	if err != nil {
		return nil, "", err
	}
	return transformed, joinPath(upstream.BasePath, endpoint), nil
}

func joinPath(basePath, endpoint string) string {
	if basePath == "/" {
		return endpoint
	}
	return strings.TrimSuffix(basePath, "/") + endpoint
}

func rejectionResponse(rejection rejection) LocalResponse {
	headers := map[string]string{"content-type": jsonContentType}
	if rejection.allow != "" {
		headers["allow"] = rejection.allow
	}
	body := openai.EncodeError(openai.ErrorDetail{
		Message: rejection.message,
		Type:    rejection.typ,
		Param:   optionalString(rejection.param),
		Code:    rejection.code,
	})
	return LocalResponse{
		StatusCode: rejection.statusCode,
		Headers:    headers,
		Body:       string(body),
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
