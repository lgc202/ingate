// Package server 装配 AI Proxy 的 Kratos transport
package server

import (
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"google.golang.org/grpc"

	"github.com/lgc202/ingate/internal/aiproxy/conf"
	"github.com/lgc202/ingate/internal/aiproxy/extproc"
	"github.com/lgc202/ingate/internal/pkg/aiproxyconfig"
)

// NewGRPCServer 创建并注册 Envoy ExtProc 服务
func NewGRPCServer(config *conf.Server, processor *extproc.Server) *kratosgrpc.Server {
	grpcConfig := config.GetGrpc()
	grpcServer := kratosgrpc.NewServer(
		kratosgrpc.Network(tcpNetwork),
		kratosgrpc.Address(grpcConfig.GetAddr()),
		kratosgrpc.DisableReflection(),
		kratosgrpc.Options(
			grpc.MaxRecvMsgSize(aiproxyconfig.ResponseBufferLimitBytes),
			grpc.MaxSendMsgSize(aiproxyconfig.ResponseBufferLimitBytes),
		),
	)
	extprocv3.RegisterExternalProcessorServer(grpcServer, processor)
	return grpcServer
}
