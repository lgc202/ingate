package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

func requestIDFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		id := request.Header.Get(requestid.Header)
		if id == "" {
			id = requestid.New()
			request.Header.Set(requestid.Header, id)
		}
		writer.Header().Set(requestid.Header, id)
		next.ServeHTTP(writer, request)
	})
}

func recoveryMiddleware(logger *slog.Logger) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (reply any, err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(ctx, "panic recovered",
						"panic", recovered,
						"stack", string(debug.Stack()),
					)
					err = kratoserrors.InternalServer(adminv1.ErrorReason_PANIC.String(), "request failed").
						WithMetadata(map[string]string{userMessageMetadata: "请求处理失败"})
				}
			}()
			return next(ctx, request)
		}
	}
}

func requestLoggingMiddleware(logger *slog.Logger) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			startedAt := time.Now()
			reply, err := next(ctx, request)
			code := http.StatusOK
			reason := ""
			operation := ""
			id := ""
			if tr, ok := transport.FromServerContext(ctx); ok {
				operation = tr.Operation()
				id = tr.RequestHeader().Get(requestid.Header)
			}
			if serviceError := kratoserrors.FromError(err); serviceError != nil {
				code = int(serviceError.Code)
				reason = serviceError.Reason
			}
			attrs := []any{
				"operation", operation,
				"code", code,
				"reason", reason,
				"latency", time.Since(startedAt),
				"request_id", id,
			}
			if code >= http.StatusInternalServerError {
				if cause := errors.Unwrap(err); cause != nil {
					attrs = append(attrs, "error", cause)
				}
				logger.ErrorContext(ctx, "server request", attrs...)
			} else {
				// 参数和业务规则错误属于正常请求结果，不按服务异常记录
				logger.InfoContext(ctx, "server request", attrs...)
			}
			return reply, err
		}
	}
}
