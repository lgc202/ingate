package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"buf.build/go/protovalidate"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	kratosvalidate "github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// maxRequestBodyBytes 覆盖现有资源和证书配置，同时限制恶意请求造成的内存占用
const maxRequestBodyBytes int64 = 4 << 20

// requestLog 保存一次管理请求需要写入日志的最小信息，不包含请求参数和响应内容
type requestLog struct {
	operation string
	requestID string
	code      int
	reason    string
	latency   time.Duration
	cause     error
}

func newRequestLog(ctx context.Context, latency time.Duration, err error) requestLog {
	entry := requestLog{
		code:    http.StatusOK,
		latency: latency,
	}
	if tr, ok := transport.FromServerContext(ctx); ok {
		entry.operation = tr.Operation()
		entry.requestID = tr.RequestHeader().Get(requestid.Header)
	}
	if serviceError := kratoserrors.FromError(err); serviceError != nil {
		entry.code = int(serviceError.Code)
		entry.reason = serviceError.Reason
	}
	if entry.code < http.StatusInternalServerError {
		return entry
	}

	if serviceError, ok := errors.AsType[*kratoserrors.Error](err); ok {
		entry.cause = errors.Unwrap(serviceError)
	} else {
		entry.cause = err
	}
	return entry
}

func (l requestLog) write(ctx context.Context, logger *slog.Logger) {
	attrs := []slog.Attr{
		slog.String("operation", l.operation),
		slog.Int("code", l.code),
		slog.String("reason", l.reason),
		slog.Duration("latency", l.latency),
		slog.String("request_id", l.requestID),
	}
	level := slog.LevelInfo
	if l.code >= http.StatusInternalServerError {
		level = slog.LevelError
		if l.cause != nil {
			attrs = append(attrs, slog.Any("err", l.cause))
		}
	}
	logger.LogAttrs(ctx, level, "server request", attrs...)
}

// httpMiddleware 按请求从外到内的执行顺序装配管理面中间件
func httpMiddleware(logger *slog.Logger) []middleware.Middleware {
	return []middleware.Middleware{
		recoveryMiddleware(logger),
		requestLoggingMiddleware(logger),
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

func requestBodyLimitFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
		next.ServeHTTP(writer, request)
	})
}

func recoveryMiddleware(logger *slog.Logger) middleware.Middleware {
	// Kratos recovery 会记录完整请求；管理请求可能包含证书私钥，因此只记录 panic 和堆栈
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
	// Kratos logging 会序列化完整请求；管理面日志只保留操作、结果和链路标识
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			startedAt := time.Now()
			reply, err := next(ctx, request)

			newRequestLog(ctx, time.Since(startedAt), err).write(ctx, logger)
			return reply, err
		}
	}
}
