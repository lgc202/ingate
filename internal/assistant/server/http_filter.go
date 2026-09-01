package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// responseState 让恢复逻辑知道响应是否已经开始。
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

// recoveryFilter 只记录最小请求字段，不记录 Header 或响应内容。
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
