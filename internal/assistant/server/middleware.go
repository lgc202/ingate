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
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/proto"

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

func requestIDFilter() kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			id := request.Header.Get(requestid.Header)
			if id == "" {
				id = requestid.New()
			}
			request.Header.Set(requestid.Header, id)
			response.Header().Set(requestid.Header, id)
			next.ServeHTTP(response, request)
		})
	}
}

// recoveryFilter 覆盖普通 API 和自定义 SSE 路由，且不会像 Kratos 默认实现一样记录请求内容。
func recoveryFilter(logger *slog.Logger) kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			state := &responseState{ResponseWriter: response}
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(request.Context(), "assistant request panic recovered",
						"request_id", request.Header.Get(requestid.Header),
						"method", request.Method,
						"path", request.URL.Path,
						"panic", recovered,
						"stack", string(debug.Stack()),
					)
					// SSE 已经发送响应头后不能再编码一份 HTTP 错误，只能结束连接。
					if !state.wroteHeader {
						kratoshttp.DefaultErrorEncoder(state, request,
							kratoserrors.InternalServer("PANIC", "request failed"))
					}
				}
			}()
			next.ServeHTTP(state, request)
		})
	}
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
			serviceError := kratoserrors.FromError(err)
			attrs := []slog.Attr{
				slog.Int("code", int(serviceError.Code)),
				slog.String("reason", serviceError.Reason),
				slog.Duration("latency", time.Since(startedAt)),
			}
			if tr, ok := transport.FromServerContext(ctx); ok {
				attrs = append(attrs,
					slog.String("operation", tr.Operation()),
					slog.String("actor", tr.RequestHeader().Get(forwardedUserHeader)),
					slog.String("request_id", tr.RequestHeader().Get(requestid.Header)),
				)
			}
			if typed, ok := errors.AsType[*kratoserrors.Error](err); ok {
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

// responseState 让恢复逻辑知道响应是否已经开始，同时保留 SSE 需要的 Flush 能力。
type responseState struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *responseState) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseState) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *responseState) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *responseState) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
