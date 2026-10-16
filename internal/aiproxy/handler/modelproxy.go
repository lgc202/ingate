// Package handler 适配 AI Proxy 的入站协议，不拥有配置加载或网络连接生命周期。
package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/aiproxy/errno"
	"github.com/lgc202/ingate/internal/aiproxy/service/modelproxy"
)

// modelOperation 是两个已注册模型操作的 Service 方法，不是可动态注册的处理链。
type modelOperation func(context.Context, modelproxy.RequestParams) (*http.Response, error)

// ModelProxyHandler 读取模型请求并调用完整 Service 用例，将上游响应流式写回客户端。
// 本 Handler 不创建 Transport、不注册路由，也不实现 Envoy 入口/出口身份协议。
type ModelProxyHandler struct {
	service             *modelproxy.Service
	maxRequestBodyBytes int64
	logger              *slog.Logger
}

// NewModelProxy 保存已构造的模型用例和明确的请求体字节上限，不设置猜测性的默认值。
// Server 注册 Chat Completions 和 Responses 创建路径，并负责 ErrorEncoder。
// 请求错误与流写回使用注入的组件 Logger，不修改或依赖全局 Logger。
func NewModelProxy(
	service *modelproxy.Service,
	maxRequestBodyBytes int64,
	logger *slog.Logger,
) (*ModelProxyHandler, error) {
	if maxRequestBodyBytes <= 0 {
		return nil, errors.New("model request body limit must be positive")
	}

	return &ModelProxyHandler{service: service, maxRequestBodyBytes: maxRequestBodyBytes, logger: logger}, nil
}

// ChatCompletion 使用 Kratos 原生 HTTP Handler 签名，保留普通响应和流式响应的写入接口。
// 入站适配层须先设置可信入口；失败返回安全的协议错误，由 Server 编码，不在此写错误响应。
// 请求体只做有界读取，具体 JSON 校验和配置捕获由 Service 完成。
func (h *ModelProxyHandler) ChatCompletion(ctx kratoshttp.Context) error {
	return h.proxy(ctx, h.service.ChatCompletion)
}

// CreateResponse 使用相同的可信入口、中间件与有界读取，响应保持 Responses 的原始格式。
func (h *ModelProxyHandler) CreateResponse(ctx kratoshttp.Context) error {
	return h.proxy(ctx, h.service.CreateResponse)
}

func (h *ModelProxyHandler) proxy(ctx kratoshttp.Context, operation modelOperation) error {
	// 原生 HTTP 消息不绑定 protobuf；整个读取与转发阶段仍须经过中间件，
	// 保证鉴权先于正文读取，并将中间件派生的上下文传到 Service 和后续阶段。
	process := ctx.Middleware(func(requestContext context.Context, _ any) (any, error) {
		return nil, h.proxyRequest(requestContext, ctx.Response(), ctx.Request(), operation)
	})
	_, err := process(ctx, nil)

	return err
}

func (h *ModelProxyHandler) proxyRequest(
	ctx context.Context,
	response http.ResponseWriter,
	request *http.Request,
	operation modelOperation,
) error {
	if err := ctx.Err(); err != nil {
		return toModelTransportError(ctx, err, h.logger)
	}

	entry, ok := ctx.Value(modelEntryKey{}).(modelEntry)
	if !ok {
		return errno.ErrModelEntryUnavailable
	}

	for _, encoding := range request.Header.Values("Content-Encoding") {
		if value := strings.TrimSpace(encoding); value != "" && !strings.EqualFold(value, "identity") {
			return errno.ErrModelEncodingUnsupported
		}
	}

	body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, h.maxRequestBodyBytes))
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return errno.ErrModelRequestTooLarge.WithCause(err)
		}

		if err := ctx.Err(); err != nil {
			return toModelTransportError(ctx, err, h.logger)
		}

		return errno.ErrModelRequestReadFailed.WithCause(err)
	}

	upstream, err := operation(ctx, modelproxy.RequestParams{
		ListenerPort: entry.port,
		HostName:     entry.hostName,
		Body:         body,
		Accept:       request.Header.Values("Accept"),
	})
	if err != nil {
		return toModelTransportError(ctx, err, h.logger)
	}

	writeModelResponse(ctx, response, upstream, h.logger)

	return nil
}

func toModelTransportError(ctx context.Context, err error, logger *slog.Logger) error {
	switch {
	case errors.Is(err, modelproxy.ErrInvalidArgument):
		return errno.ErrModelInvalidArgument.WithCause(err)
	case errors.Is(err, modelproxy.ErrModelNotFound):
		return errno.ErrModelNotFound.WithCause(err)
	case errors.Is(err, modelproxy.ErrUnavailable):
		return errno.ErrModelUnavailable.WithCause(err)
	case errors.Is(err, context.Canceled):
		return errno.ErrRequestCanceled.WithCause(err)
	case errors.Is(err, context.DeadlineExceeded):
		return errno.ErrDeadlineExceeded.WithCause(err)
	case errors.Is(err, modelproxy.ErrForwardFailed):
		logger.ErrorContext(ctx, "AI Proxy model forwarding failed", "err", err)
		return errno.ErrModelForwardFailed.WithCause(err)
	default:
		// 未知错误只在此入站边界记录；Service 不打印，Server 只编码安全的公开结果。
		logger.ErrorContext(ctx, "AI Proxy model request failed", "err", err)
		return errno.ErrInternal.WithCause(err)
	}
}
