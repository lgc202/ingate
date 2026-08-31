package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/pkg/adminidentity"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// requestLog 保存一次管理请求需要写入日志的最小信息，不包含请求参数和响应内容。
type requestLog struct {
	operation string
	actor     string
	requestID string
	code      int
	reason    string
	latency   time.Duration
	cause     error
	canceled  bool
}

func requestLoggingMiddleware(logger *slog.Logger) middleware.Middleware {
	// Kratos logging 会序列化完整请求；管理面日志只保留操作、结果和链路标识。
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			startedAt := time.Now()
			reply, err := next(ctx, request)

			newRequestLog(ctx, time.Since(startedAt), err).write(ctx, logger)
			return reply, err
		}
	}
}

func newRequestLog(ctx context.Context, latency time.Duration, err error) requestLog {
	entry := requestLog{
		code:    http.StatusOK,
		latency: latency,
	}
	if serverTransport, ok := transport.FromServerContext(ctx); ok {
		entry.operation = serverTransport.Operation()
		entry.actor = serverTransport.RequestHeader().Get(adminidentity.Header)
		entry.requestID = serverTransport.RequestHeader().Get(requestid.Header)
	}
	if ctx.Err() != nil &&
		(errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		entry.canceled = true
		return entry
	}
	if serviceError := kerrors.FromError(err); serviceError != nil {
		entry.code = int(serviceError.Code)
		if isAdminError(serviceError) {
			entry.reason = serviceError.Reason
		}
	}
	if entry.code >= http.StatusInternalServerError {
		if serviceError, ok := errors.AsType[*kerrors.Error](err); ok {
			entry.cause = errors.Unwrap(serviceError)
		} else {
			entry.cause = err
		}
	}
	return entry
}

func (l requestLog) write(ctx context.Context, logger *slog.Logger) {
	if l.canceled {
		return
	}
	// 健康检查由容器编排系统高频调用，成功结果不提供额外排障价值。
	if l.operation == adminv1.OperationHealthServiceCheck && l.code < http.StatusBadRequest {
		return
	}
	attrs := []slog.Attr{
		slog.String("operation", l.operation),
		slog.String("actor", l.actor),
		slog.Int("code", l.code),
		slog.String("reason", l.reason),
		slog.Duration("latency", l.latency),
		slog.String("request_id", l.requestID),
	}
	// 正常查询和页面轮询只在排障时需要；请求错误仍保留在默认 INFO 日志中。
	level := slog.LevelDebug
	if l.code >= http.StatusBadRequest {
		level = slog.LevelInfo
		if l.cause != nil {
			attrs = append(attrs, slog.Any("err", l.cause))
		}
	}
	if l.code >= http.StatusInternalServerError {
		level = slog.LevelError
	}
	logger.LogAttrs(ctx, level, "server request", attrs...)
}
