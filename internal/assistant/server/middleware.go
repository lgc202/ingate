package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"buf.build/go/protovalidate"
	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	kratosvalidate "github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/protobuf/proto"

	"github.com/lgc202/ingate/internal/pkg/adminidentity"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

func httpMiddleware(logger *slog.Logger) []middleware.Middleware {
	return []middleware.Middleware{
		requestErrorLoggingMiddleware(logger),
		kratosvalidate.Validator(validateRequest),
	}
}

func validateRequest(value any) error {
	message, ok := value.(proto.Message)
	if !ok {
		return nil
	}
	return protovalidate.Validate(message)
}

func requestErrorLoggingMiddleware(logger *slog.Logger) middleware.Middleware {
	// 默认 logging 中间件会序列化提示词；这里只记录失败请求的最小排障字段。
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			startedAt := time.Now()
			reply, err := next(ctx, request)
			if err == nil {
				return reply, nil
			}
			// 浏览器切换会话或结束轮询会取消正在进行的请求。这是连接生命周期，
			// 不是服务故障，也不应作为 INFO/ERROR 请求日志持续污染输出。
			if ctx.Err() != nil &&
				(errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return reply, err
			}
			serviceError := kerrors.FromError(err)
			attrs := []slog.Attr{
				slog.Int("code", int(serviceError.Code)),
				slog.String("reason", serviceError.Reason),
				slog.Duration("latency", time.Since(startedAt)),
			}
			if tr, ok := transport.FromServerContext(ctx); ok {
				attrs = append(attrs,
					slog.String("operation", tr.Operation()),
					slog.String("actor", tr.RequestHeader().Get(adminidentity.Header)),
					slog.String("request_id", tr.RequestHeader().Get(requestid.Header)),
				)
			}
			if typed, ok := errors.AsType[*kerrors.Error](err); ok {
				if cause := errors.Unwrap(typed); cause != nil {
					attrs = append(attrs, slog.Any("err", cause))
				}
			}
			level := slog.LevelInfo
			if serviceError.Code >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			logger.LogAttrs(ctx, level, "assistant request failed", attrs...)
			return reply, err
		}
	}
}
