package server

import (
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	accounthandler "github.com/lgc202/ingate/internal/adminapi/handler/account"
	backendhandler "github.com/lgc202/ingate/internal/adminapi/handler/backend"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	routehandler "github.com/lgc202/ingate/internal/adminapi/handler/httproute"
)

// Handlers 保存 Admin API 需要注册的业务 Handler。
type Handlers struct {
	Gateway   *gatewayhandler.Handler
	HTTPRoute *routehandler.Handler
	Backend   *backendhandler.Handler
	Account   *accounthandler.Handler
}

// New 构造并注册 Admin API 的入站服务，但不开始监听端口。
func New(
	config *conf.Server,
	handlers Handlers,
	chain MiddlewareChain,
) (*kratoshttp.Server, *kratosgrpc.Server) {
	httpServer := newHTTP(config.GetHttp(), chain.middlewares...)
	grpcServer := newGRPC(config.GetGrpc(), chain.middlewares...)

	adminv1.RegisterAccountServiceHTTPServer(httpServer, handlers.Account)
	adminv1.RegisterAccountServiceServer(grpcServer, handlers.Account)

	adminv1.RegisterGatewayServiceHTTPServer(httpServer, handlers.Gateway)
	adminv1.RegisterGatewayServiceServer(grpcServer, handlers.Gateway)

	adminv1.RegisterHTTPRouteServiceHTTPServer(httpServer, handlers.HTTPRoute)
	adminv1.RegisterHTTPRouteServiceServer(grpcServer, handlers.HTTPRoute)

	adminv1.RegisterBackendServiceHTTPServer(httpServer, handlers.Backend)
	adminv1.RegisterBackendServiceServer(grpcServer, handlers.Backend)

	return httpServer, grpcServer
}
