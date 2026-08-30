package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// responseState 记录响应是否已经开始，并保留代理流式响应需要的 Flush 能力。
type responseState struct {
	http.ResponseWriter
	headerWritten bool
}

func (w *responseState) WriteHeader(statusCode int) {
	if w.headerWritten {
		return
	}
	w.headerWritten = true
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseState) Write(data []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *responseState) Flush() {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *responseState) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func requestID() kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			id := requestid.GetOrCreate(request.Header)
			response.Header().Set(requestid.Header, id)
			next.ServeHTTP(response, request)
		})
	}
}

func recovery(logger *slog.Logger) kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			state := &responseState{ResponseWriter: response}
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(request.Context(),
						"console request panic recovered",
						"request_id", request.Header.Get(requestid.Header),
						"method", request.Method,
						"path", request.URL.Path,
						"panic", recovered,
						"stack", string(debug.Stack()),
					)
					if !state.headerWritten {
						writeResponse(state, http.StatusInternalServerError, "服务暂时不可用", nil)
					}
				}
			}()
			next.ServeHTTP(state, request)
		})
	}
}
