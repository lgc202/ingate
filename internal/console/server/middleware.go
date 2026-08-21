package server

import (
	"log/slog"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

func requestID() kratoshttp.FilterFunc {
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

func cors() kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			response.Header().Set("Access-Control-Allow-Origin", "*")
			response.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			response.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,"+requestid.Header)
			response.Header().Set("Access-Control-Expose-Headers", requestid.Header)
			if request.Method == http.MethodOptions {
				response.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}

func recovery(logger *slog.Logger) kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error(
						"console request panic recovered",
						"request_id", request.Header.Get(requestid.Header),
						"method", request.Method,
						"path", request.URL.Path,
						"err", recovered,
					)
					if err := writeJSON(response, http.StatusInternalServerError, map[string]any{
						"code": http.StatusInternalServerError,
						"msg":  "服务暂时不可用",
						"data": nil,
					}); err != nil {
						logger.Error("write panic response failed", "err", err)
					}
				}
			}()
			next.ServeHTTP(response, request)
		})
	}
}
