// Package mcpproxy 编排单后端 MCP Streamable HTTP 请求的入口解析、精确路径选择及 Envoy 出站调用。
// 本包透明转发所选版本的 MCP 消息，不终止协议会话、只校验 JSON-RPC 信封、不执行工具。
package mcpproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/mcpprotocol"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

var (
	// ErrRouteNotFound 表示可信入口没有匹配请求路径的 MCP 路由。
	ErrRouteNotFound = errors.New("MCP route not found")
	// ErrUnavailable 表示尚无可用配置，或选中的后端不可用。
	ErrUnavailable = errors.New("MCP service unavailable")
	// ErrForwardFailed 表示调用 Envoy 出口失败，原因链保留网络或取消错误。
	ErrForwardFailed = errors.New("MCP forwarding failed")
)

// RequestParams 保存一次 MCP 转发的可信入口、原始 HTTP 方法和原始 Header。
// Body 在调用期间由调用方保持不变，读取大小限制由入站适配层负责。
type RequestParams struct {
	ListenerPort int32
	// HostName 是 Listener 下的完整编译标识，不是客户端的 Host Header。
	HostName string
	Method   string
	// Path 是入站 URL 的路径部分，用于精确匹配 MCPRoute；查询参数不参与选择。
	Path   string
	Header http.Header
	Body   []byte
}

// Service 使用一次捕获的只读配置完成 MCP 转发，Client 只能连接受限 Envoy 出口。
type Service struct {
	loader *aiconfig.Loader
	client *http.Client
}

// New 注入已构造的 Loader 和 envoyclient.New 返回的原生 Client。
// 二者的运行、连接池清理及停止顺序由组件组合根负责。
func New(loader *aiconfig.Loader, client *http.Client) *Service {
	return &Service{loader: loader, client: client}
}

// Forward 用可信入口和请求路径选择单后端目标，再把原始 MCP 消息转发到固定 Envoy 出口。
// 调用方须先确定可信入口；本方法不根据请求内容或客户端 Header 推断入口。
// 一次调用只捕获一份 Go 配置；校验信封但不改写 JSON-RPC 正文，也不保存会话状态。
// 失败返回 nil；已选后端不可用时拒绝，不尝试其他候选或普通 HTTP。
// 成功交付响应 Body 的所有权，调用方须流式读取并关闭，不能将整个 SSE 缓存到内存。
// 取消传播到出站请求及响应读取；入站 Handler 只允许 POST。
func (s *Service) Forward(ctx context.Context, params RequestParams) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	config, loaded := s.loader.Capture()
	if !loaded {
		return nil, ErrUnavailable
	}

	backend, err := config.ResolveMCP(params.ListenerPort, params.HostName, params.Path)
	if err != nil {
		switch {
		case errors.Is(err, aiconfig.ErrMCPRouteNotFound):
			return nil, fmt.Errorf("%w: %w", ErrRouteNotFound, err)
		case errors.Is(err, aiconfig.ErrMCPBackendUnavailable):
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		default:
			return nil, err
		}
	}

	if err := mcpprotocol.Validate(params.Header, params.Body); err != nil {
		return nil, err
	}

	request, err := newMCPRequest(ctx, params, backend)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrForwardFailed, err)
	}

	if response.StatusCode == http.StatusSwitchingProtocols {
		// MCP 没有请求协议升级；不能把上游双向连接当作普通正文交付。
		cause := errors.Join(errors.New("unexpected MCP protocol upgrade"), response.Body.Close())
		return nil, fmt.Errorf("%w: %w", ErrForwardFailed, cause)
	}

	return response, nil
}

// newMCPRequest 只使用已选上游路径构造出站请求，不采用客户端 URL、查询参数或入站 Host。
func newMCPRequest(ctx context.Context, params RequestParams, backend aiconfig.MCPBackend) (*http.Request, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		params.Method,
		"http://ingate-ai-egress"+backend.BackendPath,
		bytes.NewReader(params.Body),
	)
	if err != nil {
		return nil, fmt.Errorf("build MCP request: %w", err)
	}

	request.GetBody = nil
	request.Header = forwardableMCPHeaders(params.Header)
	request.Header.Set(aidataplane.TargetHeader, backend.TargetName)

	return request, nil
}

// forwardableMCPHeaders 复制端到端请求字段，移除逐跳字段、客户端凭据和内部路由字段。
// 不继承入站 Host、分帧、Trailer 或 Envoy 控制 Header；目标 Header 由调用方重写。
// 保留 Origin、MCP 协议 Header 与未知扩展 Header，交由后端按所选版本解释。
func forwardableMCPHeaders(source http.Header) http.Header {
	headers := source.Clone()
	// 不允许客户端通过 Envoy 控制字段影响固定出口的重试、超时或路由。
	for name := range headers {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-envoy-") || strings.HasPrefix(lower, "x-forwarded-") || lower == "forwarded" {
			delete(headers, name)
		}
	}

	for _, connection := range source.Values("Connection") {
		for name := range strings.SplitSeq(connection, ",") {
			headers.Del(strings.TrimSpace(name))
		}
	}

	for _, name := range []string{
		"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
		"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
		"Content-Length", "Host", "Authorization", "Cookie",
		aidataplane.ListenerPortHeader, aidataplane.HostNameHeader, aidataplane.EntryProtocolHeader, aidataplane.TargetHeader,
	} {
		headers.Del(name)
	}

	return headers
}
