// Package runtime 编译并执行 AI Proxy 插件运行时配置
package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/anthropic"
	"github.com/lgc202/ingate/pkg/llm/gemini"
	"github.com/lgc202/ingate/pkg/llm/openai"
	"github.com/lgc202/ingate/pkg/llm/sse"
	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
	"github.com/lgc202/ingate/plugins/aiproxy/internal/policy"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
)

const (
	// MaxRequestBodyBytes 限制第一版文本请求的缓冲大小，避免为共享 Listener 扩大内存边界
	MaxRequestBodyBytes = 1 << 20
	// MaxResponseBodyBytes 限制需要整体转换的普通模型响应，SSE 不整体缓冲但会限制单个未完成事件
	MaxResponseBodyBytes = config.MaxResponseBodyBytes
	jsonContentType      = "application/json"
)

type errorEnvelope struct {
	Error errorResponse `json:"error"`
}

type errorResponse struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    string  `json:"code"`
}

// Runtime 是 AI Proxy 插件配置编译后的请求执行计划
type Runtime struct {
	runner *policy.Runner
	routes pluginruntime.RouteIndex[Route]
}

// Route 表示单条 RouteRule 的预编译模型和目标索引
type Route struct {
	Config  config.RouteConfig
	Targets map[string]config.TargetConfig
	Models  map[string]config.ModelConfig
}

// Mutation 表示发送给模型 Upstream 前需要应用的请求变更
type Mutation struct {
	Body          []byte
	Path          string
	Cluster       string
	RouteConfigID string
	Headers       []config.HeaderConfig
}

// ResponsePlan 保存当前请求的响应协议转换信息
type ResponsePlan struct {
	Protocol    config.Protocol
	PublicModel string
	Stream      bool
}

// Result 表示 AI Proxy runtime 对当前请求的处理结果
type Result struct {
	Action       pluginruntime.Action
	Mutation     Mutation
	ResponsePlan *ResponsePlan
}

// ResponseStream 是三种上游协议 SSE 转换器的具体联合类型
type ResponseStream struct {
	protocol  config.Protocol
	openAI    *openai.Stream
	anthropic *anthropic.Stream
	gemini    *gemini.Stream
}

// Compile 将插件配置转换成请求路径上可直接使用的执行计划
func Compile(cfg config.PluginConfig, runner *policy.Runner) *Runtime {
	routes := make([]Route, 0, len(cfg.Routes))
	for _, routeConfig := range cfg.Routes {
		targets := make(map[string]config.TargetConfig, len(routeConfig.Targets))
		for _, target := range routeConfig.Targets {
			targets[target.ID] = target
		}
		models := make(map[string]config.ModelConfig, len(routeConfig.Models))
		for _, model := range routeConfig.Models {
			models[model.Model] = model
		}
		routes = append(routes, Route{Config: routeConfig, Targets: targets, Models: models})
	}

	return &Runtime{
		runner: runner,
		routes: pluginruntime.NewRouteIndex(routes, func(route Route) pluginruntime.RouteKey {
			return pluginruntime.RouteKey{
				GatewayName: route.Config.GatewayName,
				RouteName:   route.Config.RouteName,
				RuleName:    route.Config.RuleName,
				ConfigID:    route.Config.ConfigID,
			}
		}),
	}
}

// Route 返回命中当前 xDS route identity 的 AI Proxy 执行配置
func (r *Runtime) Route(key pluginruntime.RouteKey) (Route, bool) {
	return r.routes.Get(key)
}

// ValidateEndpoint 校验请求是否属于第一版支持的 OpenAI API
func (r *Runtime) ValidateEndpoint(method, path string) Result {
	rejection := r.runner.ValidateEndpoint(policy.Request{Method: method, Path: path})
	if rejection != nil {
		return Result{Action: rejectionAction(rejection)}
	}
	return Result{Action: pluginruntime.Continue()}
}

// Apply 根据请求体选择目标，并生成协议、路径、Cluster 和认证变更
func (r *Runtime) Apply(route Route, body []byte) Result {
	decision := r.runner.Apply(route.Models, policy.Request{Body: body})
	if decision.Rejection != nil {
		return Result{Action: rejectionAction(decision.Rejection)}
	}
	selection := decision.Selection
	target, exists := route.Targets[selection.Model.TargetID]
	if !exists {
		return r.InternalError()
	}

	transformedBody, upstreamPath, err := transformRequest(target, selection.Model.UpstreamModel, selection.Stream, body)
	if err != nil {
		if errors.Is(err, llm.ErrInvalidRequest) || errors.Is(err, llm.ErrUnsupportedFeature) {
			return Result{Action: rejectionAction(&policy.Rejection{
				StatusCode: 400,
				Message:    err.Error(),
				Type:       "invalid_request_error",
				Code:       "unsupported_request",
			})}
		}
		return r.InternalError()
	}
	headers := make([]config.HeaderConfig, 0, len(target.Headers)+1)
	headers = append(headers, target.Headers...)
	if target.APIKey != "" {
		headers = append(headers, config.HeaderConfig{
			Name:  target.APIKeyHeader,
			Value: target.APIKeyPrefix + target.APIKey,
		})
	}
	return Result{
		Action: pluginruntime.Continue(),
		Mutation: Mutation{
			Body:          transformedBody,
			Path:          upstreamPath,
			Cluster:       target.Cluster,
			RouteConfigID: route.Config.ConfigID,
			Headers:       headers,
		},
		ResponsePlan: &ResponsePlan{
			Protocol:    target.Protocol,
			PublicModel: selection.Model.Model,
			Stream:      selection.Stream,
		},
	}
}

// TransformResponse 把普通响应或上游错误统一转换为 OpenAI-compatible JSON
func (r *Runtime) TransformResponse(plan ResponsePlan, statusCode int, body []byte) ([]byte, error) {
	if statusCode >= 400 {
		switch plan.Protocol {
		case config.ProtocolOpenAI:
			return openai.TransformError(body, statusCode), nil
		case config.ProtocolAnthropic:
			return anthropic.TransformError(body, statusCode), nil
		case config.ProtocolGemini:
			return gemini.TransformError(body, statusCode), nil
		default:
			return nil, fmt.Errorf("unsupported response protocol %q", plan.Protocol)
		}
	}
	switch plan.Protocol {
	case config.ProtocolOpenAI:
		return openai.TransformResponse(body, plan.PublicModel)
	case config.ProtocolAnthropic:
		return anthropic.TransformResponse(body, plan.PublicModel)
	case config.ProtocolGemini:
		return gemini.TransformResponse(body, plan.PublicModel)
	default:
		return nil, fmt.Errorf("unsupported response protocol %q", plan.Protocol)
	}
}

// NewResponseStream 创建当前请求对应的 SSE 转换器
func (r *Runtime) NewResponseStream(plan ResponsePlan) (*ResponseStream, error) {
	stream := &ResponseStream{protocol: plan.Protocol}
	var err error
	switch plan.Protocol {
	case config.ProtocolOpenAI:
		stream.openAI, err = openai.NewStream(plan.PublicModel)
	case config.ProtocolAnthropic:
		stream.anthropic, err = anthropic.NewStream(plan.PublicModel)
	case config.ProtocolGemini:
		stream.gemini, err = gemini.NewStream(plan.PublicModel)
	default:
		err = fmt.Errorf("unsupported response protocol %q", plan.Protocol)
	}
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// Push 转换一个任意边界的 SSE 网络分块
func (s *ResponseStream) Push(chunk []byte) ([]byte, error) {
	switch s.protocol {
	case config.ProtocolOpenAI:
		return s.openAI.Push(chunk)
	case config.ProtocolAnthropic:
		return s.anthropic.Push(chunk)
	case config.ProtocolGemini:
		return s.gemini.Push(chunk)
	default:
		return nil, fmt.Errorf("unsupported response protocol %q", s.protocol)
	}
}

// Finish 完成 SSE 转换并校验上游结束状态
func (s *ResponseStream) Finish() ([]byte, error) {
	switch s.protocol {
	case config.ProtocolOpenAI:
		return s.openAI.Finish()
	case config.ProtocolAnthropic:
		return s.anthropic.Finish()
	case config.ProtocolGemini:
		return s.gemini.Finish()
	default:
		return nil, fmt.Errorf("unsupported response protocol %q", s.protocol)
	}
}

// StreamError 把流转换异常编码成客户端仍可识别的 SSE 错误和结束标记
func (r *Runtime) StreamError() []byte {
	detail := llm.DefaultAPIError(502, "The upstream stream could not be processed")
	body := sse.EncodeData(llm.EncodeError(detail))
	return append(body, sse.EncodeData([]byte("[DONE]"))...)
}

// ResponseError 返回普通上游响应无法转换时的 OpenAI-compatible 错误
func (r *Runtime) ResponseError() []byte {
	return llm.EncodeError(llm.DefaultAPIError(502, "The upstream response could not be processed"))
}

// RequestTooLarge 返回请求体超过插件缓冲上限时的本地响应
func (r *Runtime) RequestTooLarge() Result {
	return Result{Action: rejectionAction(r.runner.RequestTooLarge())}
}

// InternalError 返回插件无法完成请求改写时的本地响应
func (r *Runtime) InternalError() Result {
	return Result{Action: rejectionAction(r.runner.InternalError())}
}

func transformRequest(target config.TargetConfig, upstreamModel string, stream bool, body []byte) ([]byte, string, error) {
	var (
		transformed []byte
		endpoint    string
		err         error
	)
	switch target.Protocol {
	case config.ProtocolOpenAI:
		transformed, err = openai.TransformRequest(body, upstreamModel)
		endpoint = openai.ChatCompletionsPath
	case config.ProtocolAnthropic:
		transformed, err = anthropic.TransformRequest(body, upstreamModel)
		endpoint = anthropic.MessagesPath
	case config.ProtocolGemini:
		transformed, err = gemini.TransformRequest(body)
		if err == nil {
			endpoint, err = gemini.EndpointPath(upstreamModel, stream)
		}
	default:
		err = fmt.Errorf("unsupported request protocol %q", target.Protocol)
	}
	if err != nil {
		return nil, "", err
	}
	return transformed, joinPath(target.BasePath, endpoint), nil
}

func joinPath(basePath, endpoint string) string {
	if basePath == "/" {
		return endpoint
	}
	return strings.TrimSuffix(basePath, "/") + endpoint
}

func rejectionAction(rejection *policy.Rejection) pluginruntime.Action {
	headers := map[string]string{"content-type": jsonContentType}
	if rejection.Allow != "" {
		headers["allow"] = rejection.Allow
	}
	body, err := json.Marshal(errorEnvelope{
		Error: errorResponse{
			Message: rejection.Message,
			Type:    rejection.Type,
			Param:   optionalString(rejection.Param),
			Code:    rejection.Code,
		},
	})
	if err != nil {
		body = []byte(`{"error":{"message":"The request could not be processed","type":"server_error","param":null,"code":"internal_error"}}`)
	}
	return pluginruntime.Respond(rejection.StatusCode, headers, string(body))
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
