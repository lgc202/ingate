// Package server 装配 ingate-ai-extproc 的 Kratos transport
package server

import (
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	"github.com/lgc202/ingate/internal/aiextproc/conf"
	aiextprocservice "github.com/lgc202/ingate/internal/aiextproc/service"
)

// NewGRPCServer 创建并注册 Envoy External Processing 服务
func NewGRPCServer(config *conf.Server, service *aiextprocservice.Service) *kratosgrpc.Server {
	server := kratosgrpc.NewServer(
		kratosgrpc.Network("tcp"),
		kratosgrpc.Address(config.GetGrpc().GetAddr()),
	)
	extprocv3.RegisterExternalProcessorServer(server, service)
	return server
}
