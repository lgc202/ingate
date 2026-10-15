// Package modelproxy 编排模型 HTTP 请求的解析、路由选择、协议适配及 Envoy 出站调用。
// 本包使用原生 HTTP Client，不接收入站 Request/ResponseWriter，不编码公开错误。
package modelproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/endpointspec"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/translator"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

var (
	// ErrInvalidArgument 表示请求体不符合当前模型入口的解析要求。
	ErrInvalidArgument = errors.New("invalid model request argument")
	// ErrModelNotFound 表示指定入口没有匹配请求模型的路由。
	ErrModelNotFound = errors.New("model not found")
	// ErrUnavailable 表示尚无可用配置，或首个匹配路由的后端不可用。
	ErrUnavailable = errors.New("model service unavailable")
	// ErrForwardFailed 表示调用 Envoy 出口失败，原因链保留网络或取消错误。
	ErrForwardFailed = errors.New("model forwarding failed")
)

// ChatCompletionParams 保存可信入口和已经读取完毕的原始请求体。
// Body 在调用期间由调用方保持不变，读取大小限制由入站适配层负责。
type ChatCompletionParams struct {
	ListenerPort int32
	// HostName 是 Listener 下的完整编译标识，不是客户端的 Host Header。
	HostName string
	Body     []byte
	// Accept 是允许透传的响应格式协商值；其他客户端 Header 不进入本用例。
	Accept []string
}

// Service 使用一次捕获的只读配置完成模型用例，Client 只能连接受限 Envoy 出口。
type Service struct {
	loader *aiconfig.Loader
	client *http.Client
}

// New 注入已构造的 Loader 和 envoyclient.New 返回的原生 Client。
// 二者的运行、连接池清理及停止顺序由组件组合根负责。
func New(loader *aiconfig.Loader, client *http.Client) *Service {
	return &Service{loader: loader, client: client}
}

// ChatCompletion 用原始 model 选择目标，应用模型名覆盖，再请求固定 Envoy 出口。
// 调用方须先确定可信入口；本方法不根据请求内容或客户端 Header 推断入口。
// 一次调用只捕获一份 Go 配置，不比较 Envoy revision，也不重新读取领域资源。
// 失败返回 nil；已选后端不可用时拒绝，不尝试其他模型规则。
// 成功交付响应 Body 的所有权，调用方须流式读取并关闭，不能将整个 SSE 缓存到内存。
// 取消传播到出站请求及响应读取；同步 JSON 解码仍由调用方的正文大小限制约束。
func (s *Service) ChatCompletion(
	ctx context.Context,
	params ChatCompletionParams,
) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	config, loaded := s.loader.Capture()
	if !loaded {
		return nil, ErrUnavailable
	}

	request, err := endpointspec.ParseChatCompletions(params.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}

	backend, err := config.ResolveModel(params.ListenerPort, params.HostName, request.Model)
	if err != nil {
		switch {
		case errors.Is(err, aiconfig.ErrModelRouteNotFound):
			return nil, fmt.Errorf("%w: %w", ErrModelNotFound, err)
		case errors.Is(err, aiconfig.ErrModelBackendUnavailable):
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		default:
			return nil, err
		}
	}

	body, err := translator.TranslateChatCompletionOpenAIToOpenAI(params.Body, backend.ModelNameOverride)
	if err != nil {
		return nil, err
	}

	upstream, err := newChatCompletionRequest(ctx, backend, body, params.Accept)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	response, err := s.client.Do(upstream)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrForwardFailed, err)
	}
	if response.StatusCode == http.StatusSwitchingProtocols {
		// 模型入口没有请求协议升级；不能把上游双向连接当作普通正文交付。
		cause := errors.Join(errors.New("unexpected model protocol upgrade"), response.Body.Close())
		return nil, fmt.Errorf("%w: %w", ErrForwardFailed, cause)
	}

	return response, nil
}

func newChatCompletionRequest(
	ctx context.Context,
	backend aiconfig.ModelBackend,
	body []byte,
	accept []string,
) (*http.Request, error) {
	path, err := url.JoinPath("/", backend.Schema.Prefix, "chat/completions")
	if err != nil {
		return nil, fmt.Errorf("build model operation path: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://ingate-ai-egress"+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build model request: %w", err)
	}

	// 从零构造出站请求，不继承客户端 URL、认证、分帧、Trailer 或 Envoy 控制 Header。
	request.GetBody = nil
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(aidataplane.TargetHeader, backend.TargetName)
	request.Header["Accept"] = slices.Clone(accept)
	request.Header["User-Agent"] = []string{""}

	return request, nil
}
