package server

import (
	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/lgc202/ingate/internal/adminapi/conf"
)

func newGRPC(config *conf.Server_GRPC, middlewares ...kratosmiddleware.Middleware) *kratosgrpc.Server {
	return kratosgrpc.NewServer(
		kratosgrpc.Network(config.GetNetwork()),
		kratosgrpc.Address(config.GetAddr()),
		kratosgrpc.Timeout(config.GetTimeout().AsDuration()),
		kratosgrpc.Middleware(middlewares...),
	)
}
