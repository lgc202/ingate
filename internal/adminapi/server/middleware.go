package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"

	"buf.build/go/protovalidate"
	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

type panicCause struct {
	stack []byte
}

func (c panicCause) Error() string {
	return fmt.Sprintf("panic recovered\n%s", c.stack)
}

// serverMiddleware 按请求从外到内的执行顺序装配管理面中间件。
// HTTP 与 gRPC 共享相同的恢复、日志、错误脱敏和请求校验规则。
func serverMiddleware(logger *slog.Logger) []middleware.Middleware {
	return []middleware.Middleware{
		requestLoggingMiddleware(logger),
		recoveryMiddleware(),
		errorSanitizingMiddleware(),
		requestValidationMiddleware(),
	}
}

func recoveryMiddleware() middleware.Middleware {
	// Kratos recovery 会记录完整请求；管理请求可能包含证书私钥，因此只保留堆栈作为内部 cause。
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (reply any, err error) {
			defer func() {
				if recover() != nil {
					err = adminv1.ErrorPanic("请求处理失败").WithCause(
						panicCause{stack: debug.Stack()},
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
					return reply, adminv1.ErrorRequestCanceled("请求已取消").WithCause(err)
				case errors.Is(err, context.DeadlineExceeded):
					return reply, adminv1.ErrorRequestTimeout("请求处理超时").WithCause(err)
				}
			}
			serviceError, ok := errors.AsType[*kerrors.Error](err)
			if ok && isAdminError(serviceError) {
				return reply, serviceError.WithCause(err)
			}
			return reply, adminv1.ErrorInternalError("请求处理失败").WithCause(err)
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
				return nil, adminv1.ErrorInvalidArgument("请求参数不正确").WithCause(err)
			}
			return next(ctx, request)
		}
	}
}
