// Package server 装配 Admin API 的 Kratos transport
package server

import (
	"log/slog"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/adminapi/conf"
)

// NewHTTPServer 创建 Admin API 的 Kratos HTTP transport
func NewHTTPServer(
	config *conf.Server,
	logger *slog.Logger,
	handlers *HTTPHandlers,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	httpServer := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
		kratoshttp.Filter(requestIDFilter, requestBodyLimitFilter),
		kratoshttp.Middleware(httpMiddleware(logger)...),
		kratoshttp.RequestDecoder(requestDecoder),
		kratoshttp.ResponseEncoder(responseEncoder),
		kratoshttp.ErrorEncoder(errorEncoder),
		kratoshttp.NotFoundHandler(http.HandlerFunc(endpointNotFoundHandler)),
		kratoshttp.MethodNotAllowedHandler(http.HandlerFunc(methodNotAllowedHandler)),
	)
	handlers.register(httpServer)
	return httpServer
}
