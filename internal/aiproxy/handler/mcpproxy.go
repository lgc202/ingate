package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/aiproxy/errno"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/mcpprotocol"
	"github.com/lgc202/ingate/internal/aiproxy/service/mcpproxy"
)

// MCPProxyHandler 读取 MCP 请求并调用完整 Service 用例，将上游响应流式写回客户端。
// 本 Handler 在受保护入口上校验方法、Origin 与现代协议信封，不重建业务请求。
type MCPProxyHandler struct {
	service             *mcpproxy.Service
	maxRequestBodyBytes int64
	logger              *slog.Logger
}

// NewMCPProxy 保存已构造的 MCP 用例和明确的请求体字节上限，不设置猜测性的默认值。
// 请求错误与流写回使用注入的组件 Logger，不修改或依赖全局 Logger。
func NewMCPProxy(
	service *mcpproxy.Service,
	maxRequestBodyBytes int64,
	logger *slog.Logger,
) (*MCPProxyHandler, error) {
	if maxRequestBodyBytes <= 0 {
		return nil, errors.New("MCP request body limit must be positive")
	}

	return &MCPProxyHandler{service: service, maxRequestBodyBytes: maxRequestBodyBytes, logger: logger}, nil
}

// Forward 使用 Kratos 原生 HTTP Handler 签名，保留普通响应和流式响应的写入接口。
// 入站适配层须先设置可信入口；校验与转发失败在写出错误响应后返回 nil，不交给 Kratos 再编码。
func (h *MCPProxyHandler) Forward(ctx kratoshttp.Context) error {
	// 原生 HTTP 消息不绑定 protobuf；整个读取与转发阶段仍须经过中间件，
	// 保证鉴权先于正文读取，并将中间件派生的上下文传到 Service 和后续阶段。
	process := ctx.Middleware(func(requestContext context.Context, _ any) (any, error) {
		return nil, h.forwardRequest(requestContext, ctx.Response(), ctx.Request())
	})
	_, err := process(ctx, nil)
	if err != nil {
		public, ok := errors.AsType[*kratoserrors.Error](err)
		if !ok {
			public = errno.ErrInternal
		}

		code := mcpprotocol.CodeInvalidRequest
		if public.Code >= http.StatusInternalServerError {
			code = mcpprotocol.CodeInternalError
		}

		mcpprotocol.WriteError(ctx.Response(), int(public.Code), code, public.Message, nil, nil)
	}

	return nil
}

func (h *MCPProxyHandler) forwardRequest(
	ctx context.Context,
	response http.ResponseWriter,
	request *http.Request,
) error {
	if err := ctx.Err(); err != nil {
		return toMCPTransportError(ctx, err, h.logger)
	}

	entry, ok := entryFromContext(ctx)
	if !ok {
		return errno.ErrMCPEntryUnavailable
	}

	if !sameOriginMCP(request) {
		mcpprotocol.WriteError(response, http.StatusForbidden, mcpprotocol.CodeInvalidRequest, "MCP origin is not allowed", nil, nil)
		return nil
	}

	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		mcpprotocol.WriteError(response, http.StatusMethodNotAllowed, mcpprotocol.CodeInvalidRequest, "MCP entry only accepts POST", nil, nil)

		return nil
	}

	body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, h.maxRequestBodyBytes))
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return errno.ErrMCPRequestTooLarge.WithCause(err)
		}

		if err := ctx.Err(); err != nil {
			return toMCPTransportError(ctx, err, h.logger)
		}

		return errno.ErrMCPRequestReadFailed.WithCause(err)
	}

	upstream, err := h.service.Forward(ctx, mcpproxy.RequestParams{
		ListenerPort: entry.port,
		HostName:     entry.hostName,
		Method:       request.Method,
		Path:         request.URL.EscapedPath(),
		Header:       request.Header,
		Body:         body,
	})
	if err != nil {
		protocolErr, ok := errors.AsType[*mcpprotocol.Error](err)
		if ok {
			mcpprotocol.WriteError(response, http.StatusBadRequest, protocolErr.Code, protocolErr.Message, protocolErr.ID, protocolErr.Data)
			return nil
		}

		return toMCPTransportError(ctx, err, h.logger)
	}

	writeUpstreamResponse(ctx, response, upstream, h.logger)

	return nil
}

// sameOriginMCP 校验 Origin 与可信外部入口同源。可信 authority 只有 Envoy 已匹配的请求 Host；
// 不采信客户端 Forwarded/X-Forwarded-*。首版公网 Listener 只有 HTTP，因此 scheme 固定 http；
// 引入公网 TLS/scheme 契约时必须同步扩展。未使用 net/http.CrossOriginProtection：它跳过 safe
// 方法、忽略 scheme，也不规范化默认端口与大小写，不满足本入口的完整来源校验。
func sameOriginMCP(request *http.Request) bool {
	origins := request.Header.Values("Origin")
	if len(origins) == 0 {
		return true
	}

	if len(origins) != 1 {
		return false
	}

	parsed, err := url.Parse(origins[0])
	if err != nil {
		return false
	}

	if parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(origins[0], "#") {
		return false
	}

	return sameAuthority(parsed.Host, request.Host)
}

func sameAuthority(origin, authority string) bool {
	originHost, originPort, originOK := httpAuthority(origin)
	entryHost, entryPort, entryOK := httpAuthority(authority)

	return originOK && entryOK && originHost == entryHost && originPort == entryPort
}

// httpAuthority 只接受合法 HTTP authority，补齐默认端口并规范化 IP 字面量。
// net/url 负责方括号与端口语法；不从 Forwarded 等客户端字段猜测外部入口。
func httpAuthority(authority string) (string, uint64, bool) {
	parsed, err := url.Parse("http://" + authority)
	if err != nil || parsed.Host != authority || parsed.User != nil || parsed.Hostname() == "" {
		return "", 0, false
	}

	host := strings.ToLower(parsed.Hostname())
	if strings.HasPrefix(authority, "[") || strings.ContainsAny(host, " :/\\?#@%") {
		if ip, err := netip.ParseAddr(host); err == nil && ip.Zone() == "" && strings.HasPrefix(authority, "[") {
			host = ip.String()
		} else {
			return "", 0, false
		}
	}

	port := uint64(80)
	if parsed.Port() != "" {
		port, err = strconv.ParseUint(parsed.Port(), 10, 16)
		if err != nil || port == 0 {
			return "", 0, false
		}
	} else if strings.HasSuffix(authority, ":") {
		return "", 0, false
	}

	return host, port, true
}

func toMCPTransportError(ctx context.Context, err error, logger *slog.Logger) error {
	switch {
	case errors.Is(err, mcpproxy.ErrRouteNotFound):
		return errno.ErrMCPRouteNotFound.WithCause(err)
	case errors.Is(err, mcpproxy.ErrUnavailable):
		return errno.ErrMCPUnavailable.WithCause(err)
	case errors.Is(err, context.Canceled):
		return errno.ErrRequestCanceled.WithCause(err)
	case errors.Is(err, context.DeadlineExceeded):
		return errno.ErrDeadlineExceeded.WithCause(err)
	case errors.Is(err, mcpproxy.ErrForwardFailed):
		logger.ErrorContext(ctx, "AI Proxy MCP forwarding failed", "err", err)
		return errno.ErrMCPForwardFailed.WithCause(err)
	default:
		// 未知错误只在此入站边界记录；Service 不打印，Server 只编码安全的公开结果。
		logger.ErrorContext(ctx, "AI Proxy MCP request failed", "err", err)
		return errno.ErrInternal.WithCause(err)
	}
}
