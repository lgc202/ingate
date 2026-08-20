// Package server 装配 ingate-authz 的 Kratos transport
package server

import (
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	"github.com/lgc202/ingate/internal/authz/conf"
	authzservice "github.com/lgc202/ingate/internal/authz/service"
)

// NewGRPCServer 创建并注册 Envoy External Authorization 服务
func NewGRPCServer(config *conf.Server, service *authzservice.Service) *kratosgrpc.Server {
	server := kratosgrpc.NewServer(
		kratosgrpc.Network("tcp"),
		kratosgrpc.Address(config.GetGrpc().GetAddr()),
	)
	authv3.RegisterAuthorizationServer(server, service)
	return server
}
