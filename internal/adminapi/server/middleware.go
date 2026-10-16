package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/selector"
	"github.com/go-kratos/kratos/v3/transport"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminmiddleware "github.com/lgc202/ingate/internal/adminapi/middleware"
	"go.opentelemetry.io/otel/trace"
)

// MiddlewareChain 保存 HTTP 和 gRPC 共用的有序入站中间件链。
type MiddlewareChain struct {
	middlewares []kratosmiddleware.Middleware
}

// NewMiddlewareChain 构造 Admin API 的完整入站中间件链。
func NewMiddlewareChain(logger *slog.Logger, authentication kratosmiddleware.Middleware) MiddlewareChain {
	return MiddlewareChain{
		middlewares: []kratosmiddleware.Middleware{
			requestLogging(logger),
			selector.Server(authentication).Match(requiresAuthentication).Build(),
			adminmiddleware.ValidateRequests(),
		},
	}
}

// requestLogging 只记录 RPC 用例完成结果，不读取请求、响应、Header 或完整 URL。
// 认证与校验在内侧；成功和 4xx 默认为静默，5xx 保留内部原因供排障。
// 解码、404、响应写出失败和 panic 不属于 next 返回边界，不将此日志当作可靠审计。
func requestLogging(logger *slog.Logger) kratosmiddleware.Middleware {
	return func(next kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			start := time.Now()
			reply, err := next(ctx, req)

			code := kratoserrors.Code(err)
			level := slog.LevelDebug
			if code >= http.StatusInternalServerError {
				level = slog.LevelError
			}

			if !logger.Enabled(ctx, level) {
				return reply, err
			}

			var kind, operation string
			if info, ok := transport.FromServerContext(ctx); ok {
				kind = info.Kind().String()
				operation = info.Operation()
			}

			attrs := []slog.Attr{
				slog.String("kind", "server"),
				slog.String("component", kind),
				slog.String("operation", operation),
				slog.Int("code", code),
				slog.String("reason", kratoserrors.Reason(err)),
				slog.Float64("latency", time.Since(start).Seconds()),
			}
			if span := trace.SpanContextFromContext(ctx); span.IsValid() {
				attrs = append(attrs,
					slog.String("trace_id", span.TraceID().String()),
					slog.String("span_id", span.SpanID().String()),
				)
			}

			if level == slog.LevelError {
				// 4xx cause 可能含用户输入，只有服务端失败才记录内部错误链。
				attrs = append(attrs, slog.Any("error", err))
			}

			logger.LogAttrs(ctx, level, "server request", attrs...)

			return reply, err
		}
	}
}

func requiresAuthentication(_ context.Context, operation string) bool {
	return operation != adminv1.OperationAccountServiceLogin
}
