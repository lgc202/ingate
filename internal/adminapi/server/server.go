package server

import (
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	accounthandler "github.com/lgc202/ingate/internal/adminapi/handler/account"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
)

// Handlers 保存 Admin API 需要注册的业务 Handler。
type Handlers struct {
	Gateway *gatewayhandler.Handler
	Account *accounthandler.Handler
}

// New 构造并注册 Admin API 的入站服务，但不开始监听端口。
func New(
	config *conf.Server,
	handlers Handlers,
	chain MiddlewareChain,
) (*kratoshttp.Server, *kratosgrpc.Server) {
	httpServer := newHTTP(config.GetHttp(), chain.middlewares...)
	grpcServer := newGRPC(config.GetGrpc(), chain.middlewares...)

	registerAccount(httpServer, grpcServer, handlers.Account)
	registerGateway(httpServer, grpcServer, handlers.Gateway)

	return httpServer, grpcServer
}

func registerAccount(
	httpServer *kratoshttp.Server,
	grpcServer *kratosgrpc.Server,
	handler *accounthandler.Handler,
) {
	adminv1.RegisterAccountServiceHTTPServer(httpServer, handler)
	adminv1.RegisterAccountServiceServer(grpcServer, handler)
}

func registerGateway(
	httpServer *kratoshttp.Server,
	grpcServer *kratosgrpc.Server,
	handler *gatewayhandler.Handler,
) {
	adminv1.RegisterGatewayServiceHTTPServer(httpServer, handler)
	adminv1.RegisterGatewayServiceServer(grpcServer, handler)
}
