package server

import (
	"net/http"

	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/lgc202/ingate/internal/adminapi/conf"
)

func newHTTP(config *conf.Server_HTTP, middlewares ...kratosmiddleware.Middleware) *kratoshttp.Server {
	return kratoshttp.NewServer(
		kratoshttp.Network(config.GetNetwork()),
		kratoshttp.Address(config.GetAddr()),
		kratoshttp.Timeout(config.GetTimeout().AsDuration()),
		kratoshttp.Middleware(middlewares...),
		kratoshttp.RequestDecoder(decodeHTTPRequest),
		kratoshttp.ResponseEncoder(encodeHTTPResponse),
		kratoshttp.ErrorEncoder(encodeHTTPError),
		// 不回退到全局 DefaultServeMux，避免导入 pprof 后从业务端口意外暴露。
		kratoshttp.NotFoundHandler(http.NotFoundHandler()),
		kratoshttp.MethodNotAllowedHandler(http.NotFoundHandler()),
	)
}
