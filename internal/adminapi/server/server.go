package server

import (
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/handler"
)

// Handlers 保存 Admin API 需要注册的业务 Handler。
type Handlers struct {
	Gateway    *handler.GatewayHandler
	HTTPRoute  *handler.HTTPRouteHandler
	Backend    *handler.BackendHandler
	AIBackend  *handler.AIBackendHandler
	ModelRoute *handler.ModelRouteHandler
	MCPRoute   *handler.MCPRouteHandler
	Account    *handler.AccountHandler
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

	adminv1.RegisterAIBackendServiceHTTPServer(httpServer, handlers.AIBackend)
	adminv1.RegisterAIBackendServiceServer(grpcServer, handlers.AIBackend)

	adminv1.RegisterModelRouteServiceHTTPServer(httpServer, handlers.ModelRoute)
	adminv1.RegisterModelRouteServiceServer(grpcServer, handlers.ModelRoute)

	adminv1.RegisterMCPRouteServiceHTTPServer(httpServer, handlers.MCPRoute)
	adminv1.RegisterMCPRouteServiceServer(grpcServer, handlers.MCPRoute)

	return httpServer, grpcServer
}
