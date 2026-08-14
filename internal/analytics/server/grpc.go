package server

import (
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	"github.com/lgc202/ingate/internal/analytics/conf"
	requestservice "github.com/lgc202/ingate/internal/analytics/service/request"
	trafficservice "github.com/lgc202/ingate/internal/analytics/service/traffic"
)

// NewGRPCServer 创建供管理面查询请求分析结果的 gRPC 服务。
func NewGRPCServer(
	config *conf.Server,
	requestService *requestservice.Service,
	trafficService *trafficservice.Service,
) *kratosgrpc.Server {
	grpcConfig := config.GetGrpc()
	server := kratosgrpc.NewServer(
		kratosgrpc.Network("tcp"),
		kratosgrpc.Address(grpcConfig.GetAddr()),
		kratosgrpc.Timeout(grpcConfig.GetTimeout().AsDuration()),
	)
	analyticsv1.RegisterRequestServiceServer(server, requestService)
	analyticsv1.RegisterTrafficServiceServer(server, trafficService)
	return server
}
