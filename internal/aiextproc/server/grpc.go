package server

import (
	"fmt"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	aiextprocv1 "github.com/lgc202/ingate/api/aiextproc/v1"
	"github.com/lgc202/ingate/internal/aiextproc/conf"
	aiextprocservice "github.com/lgc202/ingate/internal/aiextproc/service"
	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

// NewGRPCServer 创建并注册 Envoy External Processing 服务。
func NewGRPCServer(
	config *conf.Server,
	processor *aiextprocservice.ExternalProcessor,
	quotaUsage *aiextprocservice.TokenQuotaUsageService,
) (*kratosgrpc.Server, error) {
	grpcConfig := config.GetGrpc()
	options := []kratosgrpc.ServerOption{
		kratosgrpc.Network("tcp"),
		kratosgrpc.Address(grpcConfig.GetAddr()),
	}
	tlsSettings := grpcConfig.GetTls()
	tlsConfig, err := tlsconfig.NewServer(tlsconfig.ServerConfig{
		Enabled:         tlsSettings.GetEnabled(),
		CertificateFile: tlsSettings.GetCertFile(),
		PrivateKeyFile:  tlsSettings.GetKeyFile(),
		ClientCAFile:    tlsSettings.GetClientCaFile(),
	})
	if err != nil {
		return nil, fmt.Errorf("configure AI ExtProc gRPC TLS: %w", err)
	}
	if tlsConfig != nil {
		options = append(options, kratosgrpc.TLSConfig(tlsConfig))
	}

	server := kratosgrpc.NewServer(options...)
	extprocv3.RegisterExternalProcessorServer(server, processor)
	// 额度查询与 ExtProc 复用内部 gRPC 入口，不引入只为读取 Redis 计数的新组件
	aiextprocv1.RegisterTokenQuotaUsageServiceServer(server, quotaUsage)
	return server, nil
}
