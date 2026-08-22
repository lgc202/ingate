package server

import (
	"fmt"

	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	"github.com/lgc202/ingate/internal/analytics/conf"
	aiusageservice "github.com/lgc202/ingate/internal/analytics/service/aiusage"
	requestservice "github.com/lgc202/ingate/internal/analytics/service/request"
	trafficservice "github.com/lgc202/ingate/internal/analytics/service/traffic"
	"github.com/lgc202/ingate/internal/pkg/tlsx"
)

// NewGRPCServer 创建供 Admin API 查询请求明细、流量分析和 AI 用量的 gRPC 服务
//
// Analytics 不直接向控制台浏览器开放，资源名称和前端响应组装由 Admin API 负责
func NewGRPCServer(
	config *conf.Server,
	aiUsageService *aiusageservice.Service,
	requestService *requestservice.Service,
	trafficService *trafficservice.Service,
) (*kratosgrpc.Server, error) {
	grpcConfig := config.GetGrpc()
	options := []kratosgrpc.ServerOption{
		kratosgrpc.Network("tcp"),
		kratosgrpc.Address(grpcConfig.GetAddr()),
		kratosgrpc.Timeout(grpcConfig.GetTimeout().AsDuration()),
	}
	tlsSettings := grpcConfig.GetTls()
	tlsConfig, err := tlsx.NewServer(tlsx.ServerConfig{
		Enabled:         tlsSettings.GetEnabled(),
		CertificateFile: tlsSettings.GetCertFile(),
		PrivateKeyFile:  tlsSettings.GetKeyFile(),
		ClientCAFile:    tlsSettings.GetClientCaFile(),
	})
	if err != nil {
		return nil, fmt.Errorf("configure Analytics gRPC TLS: %w", err)
	}
	if tlsConfig != nil {
		options = append(options, kratosgrpc.TLSConfig(tlsConfig))
	}
	server := kratosgrpc.NewServer(options...)
	analyticsv1.RegisterAIUsageServiceServer(server, aiUsageService)
	analyticsv1.RegisterRequestServiceServer(server, requestService)
	analyticsv1.RegisterTrafficServiceServer(server, trafficService)
	return server, nil
}
