package server

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"

	"buf.build/go/protovalidate"
	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

// serverMiddleware 按请求从外到内的执行顺序装配管理面中间件。
// HTTP 与 gRPC 共享相同的恢复、日志、错误脱敏和请求校验规则。
func serverMiddleware(logger *slog.Logger) []middleware.Middleware {
	return []middleware.Middleware{
		requestLoggingMiddleware(logger),
		recoveryMiddleware(logger),
		errorSanitizingMiddleware(),
		requestValidationMiddleware(),
	}
}

func recoveryMiddleware(logger *slog.Logger) middleware.Middleware {
	// Kratos recovery 会记录完整请求；管理请求可能包含证书私钥，因此只记录堆栈。
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (reply any, err error) {
			defer func() {
				if recover() != nil {
					logger.ErrorContext(ctx, "panic recovered",
						"stack", string(debug.Stack()),
					)
					err = kerrors.InternalServer(
						adminv1.ErrorReason_PANIC.String(),
						"请求处理失败",
					)
				}
			}()
			return next(ctx, request)
		}
	}
}

func errorSanitizingMiddleware() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			reply, err := next(ctx, request)
			if err == nil {
				return reply, nil
			}
			if ctx.Err() != nil {
				switch {
				case errors.Is(err, context.Canceled):
					return reply, kerrors.ClientClosed(
						adminv1.ErrorReason_REQUEST_CANCELED.String(),
						"请求已取消",
					).WithCause(err)
				case errors.Is(err, context.DeadlineExceeded):
					return reply, kerrors.GatewayTimeout(
						adminv1.ErrorReason_REQUEST_TIMEOUT.String(),
						"请求处理超时",
					).WithCause(err)
				}
			}
			serviceError, ok := errors.AsType[*kerrors.Error](err)
			if ok && isAdminError(serviceError) {
				return reply, kerrors.Clone(serviceError).WithCause(err)
			}
			return reply, kerrors.InternalServer(
				adminv1.ErrorReason_INTERNAL_ERROR.String(),
				"请求处理失败",
			).WithCause(err)
		}
	}
}

func requestValidationMiddleware() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			message, ok := request.(proto.Message)
			if !ok {
				return next(ctx, request)
			}
			if err := protovalidate.Validate(message); err != nil {
				return nil, kerrors.BadRequest(
					adminv1.ErrorReason_INVALID_ARGUMENT.String(),
					"请求参数不正确",
				).WithCause(err)
			}
			return next(ctx, request)
		}
	}
}
