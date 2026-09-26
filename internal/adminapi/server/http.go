package server

import (
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
	)
}
