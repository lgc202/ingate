package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// Assistant 的单条消息最多 64 KiB；1 MiB 为 Proto JSON 和后续字段保留充足余量。
const maxRequestBodyBytes int64 = 1 << 20

// responseState 让恢复逻辑知道响应是否已经开始，同时保留 SSE 需要的 Flush 能力。
type responseState struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *responseState) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(statusCode)
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

func requestIDFilter() kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			id := requestid.GetOrCreate(request.Header)
			response.Header().Set(requestid.Header, id)
			next.ServeHTTP(response, request)
		})
	}
}

func responseNoStoreFilter() kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			response.Header().Set("Cache-Control", "no-store")
			next.ServeHTTP(response, request)
		})
	}
}

func requestBodyLimitFilter() kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			request.Body = http.MaxBytesReader(response, request.Body, maxRequestBodyBytes)
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
						"stack", string(debug.Stack()),
					)
					// SSE 已经发送响应头后不能再编码一份 HTTP 错误，只能结束连接。
					if !state.wroteHeader {
						kratoshttp.DefaultErrorEncoder(state, request,
							kerrors.InternalServer("PANIC", "request failed"))
					}
				}
			}()
			next.ServeHTTP(state, request)
		})
	}
}
