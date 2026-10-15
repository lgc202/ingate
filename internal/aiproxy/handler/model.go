// Package handler 适配 AI Proxy 的入站协议，不拥有配置加载或网络连接生命周期。
package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	modelservice "github.com/lgc202/ingate/internal/aiproxy/service/model"
)

// ModelHandler 读取模型请求并调用 Service，将准备结果交给 HTTP 转发阶段。
// next 只能是受限出站适配器，须从 ModelBackend 读取目标，不得按客户端 URL 发起连接。
// 本 Handler 不创建 Transport、不注册路由，也不实现 Envoy 入口/出口身份协议。
type ModelHandler struct {
	service             *modelservice.Service
	next                http.Handler
	maxRequestBodyBytes int64
}

// NewModel 保存模型用例、后续转发阶段和明确的请求体字节上限，不设置猜测性的默认值。
// Server 后续以 POST /v1/chat/completions 注册 ChatCompletion，并负责 ErrorEncoder。
func NewModel(
	service *modelservice.Service,
	next http.Handler,
	maxRequestBodyBytes int64,
) (*ModelHandler, error) {
	if service == nil {
		return nil, errors.New("model service is required")
	}
	if next == nil {
		return nil, errors.New("model forwarding handler is required")
	}
	if maxRequestBodyBytes <= 0 {
		return nil, errors.New("model request body limit must be positive")
	}

	return &ModelHandler{service: service, next: next, maxRequestBodyBytes: maxRequestBodyBytes}, nil
}

// ChatCompletion 使用 Kratos 原生 HTTP Handler 签名，保留普通响应和流式响应的写入接口。
// 入站适配层须先设置可信入口；失败返回安全的协议错误，由 Server 编码，不在此写错误响应。
// 请求体只做有界读取，具体 JSON 校验和配置捕获由 Service 完成。
func (h *ModelHandler) ChatCompletion(ctx kratoshttp.Context) error {
	// 原生 HTTP 消息不绑定 protobuf；整个读取与转发阶段仍须经过中间件，
	// 保证鉴权先于正文读取，并将中间件派生的上下文传到 Service 和后续阶段。
	process := ctx.Middleware(func(requestContext context.Context, _ any) (any, error) {
		return nil, h.chatCompletion(requestContext, ctx.Response(), ctx.Request())
	})
	_, err := process(ctx, nil)

	return err
}

func (h *ModelHandler) chatCompletion(
	ctx context.Context,
	response http.ResponseWriter,
	request *http.Request,
) error {
	if err := ctx.Err(); err != nil {
		return toModelTransportError(ctx, err)
	}
	entry, ok := ctx.Value(modelEntryKey{}).(modelEntry)
	if !ok {
		return kratoserrors.ServiceUnavailable("MODEL_ENTRY_UNAVAILABLE", "model entry is unavailable")
	}
	for _, encoding := range request.Header.Values("Content-Encoding") {
		if value := strings.TrimSpace(encoding); value != "" && !strings.EqualFold(value, "identity") {
			return kratoserrors.New(
				http.StatusUnsupportedMediaType,
				"MODEL_ENCODING_UNSUPPORTED",
				"compressed model requests are not supported",
			)
		}
	}

	body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, h.maxRequestBodyBytes))
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return kratoserrors.New(
				http.StatusRequestEntityTooLarge,
				"MODEL_REQUEST_TOO_LARGE",
				"model request body is too large",
			).WithCause(err)
		}
		if err := ctx.Err(); err != nil {
			return toModelTransportError(ctx, err)
		}

		return kratoserrors.BadRequest("MODEL_REQUEST_READ_FAILED", "cannot read model request body").WithCause(err)
	}

	prepared, err := h.service.PrepareChatCompletion(ctx, modelservice.ChatCompletionParams{
		ListenerPort: entry.port,
		HostName:     entry.hostName,
		Body:         body,
	})
	if err != nil {
		return toModelTransportError(ctx, err)
	}

	// Clone 保留取消与追踪上下文，同时隔离 Header、URL 等可变协议状态。
	// 入站 Body 由 HTTP Server 关闭；交付的新 Body 属于后续转发阶段。
	forwardContext := context.WithValue(ctx, modelBackendKey{}, prepared.Backend)
	forwarded := request.Clone(forwardContext)
	forwarded.Body = io.NopCloser(bytes.NewReader(prepared.Body))
	forwarded.GetBody = nil // 不能重放 Clone 可能继承的原始、未改写请求体。
	forwarded.ContentLength = int64(len(prepared.Body))
	forwarded.TransferEncoding = nil
	forwarded.Trailer = nil
	forwarded.Header.Del("Content-Length")
	forwarded.Header.Del("Transfer-Encoding")
	forwarded.Header.Del("Trailer")
	forwarded.Header.Del("Content-Encoding")
	forwarded.Header.Set("Content-Type", "application/json")

	// 直接交付原 ResponseWriter，不缓冲响应或包装掉 Flusher 等能力。
	h.next.ServeHTTP(response, forwarded)

	return nil
}

func toModelTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, modelservice.ErrInvalidArgument):
		return kratoserrors.BadRequest("MODEL_INVALID_ARGUMENT", "invalid model request").WithCause(err)
	case errors.Is(err, modelservice.ErrModelNotFound):
		return kratoserrors.NotFound("MODEL_NOT_FOUND", "model is not available at this entry").WithCause(err)
	case errors.Is(err, modelservice.ErrUnavailable):
		return kratoserrors.ServiceUnavailable("MODEL_UNAVAILABLE", "model service is unavailable").WithCause(err)
	case errors.Is(err, context.Canceled):
		return kratoserrors.ClientClosed("REQUEST_CANCELED", "request was canceled").WithCause(err)
	case errors.Is(err, context.DeadlineExceeded):
		return kratoserrors.GatewayTimeout("DEADLINE_EXCEEDED", "request deadline exceeded").WithCause(err)
	default:
		// 未知错误只在此入站边界记录；Service 不打印，Server 只编码安全的公开结果。
		slog.ErrorContext(ctx, "AI Proxy model request failed", "err", err)
		return kratoserrors.InternalServer("INTERNAL_SERVER_ERROR", "internal server error").WithCause(err)
	}
}
