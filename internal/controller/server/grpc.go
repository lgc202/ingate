// Package server 装配 ingate-controller 的 Kratos transport
package server

import (
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	"github.com/lgc202/ingate/internal/controller/conf"
	"github.com/lgc202/ingate/internal/controller/xds"
)

// NewGRPCServer 创建并注册 Envoy ADS 服务
func NewGRPCServer(config *conf.Server, service *xds.Service) *kratosgrpc.Server {
	server := kratosgrpc.NewServer(
		kratosgrpc.Network("tcp"),
		kratosgrpc.Address(config.GetGrpc().GetAddr()),
	)
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(server, service)
	return server
}
