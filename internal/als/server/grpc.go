// Package server 装配 ingate-als 的 Kratos transport 和积压回放任务
package server

import (
	"fmt"

	accesslogservice "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	"github.com/lgc202/ingate/internal/als/conf"
	alsservice "github.com/lgc202/ingate/internal/als/service"
	"github.com/lgc202/ingate/pkg/tlsx"
)

// NewGRPCServer 创建并注册 Envoy ALS gRPC 服务
func NewGRPCServer(config *conf.Server, service *alsservice.Service) (*kratosgrpc.Server, error) {
	options := []kratosgrpc.ServerOption{
		kratosgrpc.Network("tcp"),
		kratosgrpc.Address(config.GetGrpc().GetAddr()),
	}
	tlsSettings := config.GetGrpc().GetTls()
	tlsConfig, err := tlsx.NewServer(tlsx.ServerConfig{
		Enabled:         tlsSettings.GetEnabled(),
		CertificateFile: tlsSettings.GetCertFile(),
		PrivateKeyFile:  tlsSettings.GetKeyFile(),
		ClientCAFile:    tlsSettings.GetClientCaFile(),
	})
	if err != nil {
		return nil, fmt.Errorf("configure ALS gRPC TLS: %w", err)
	}
	if tlsConfig != nil {
		options = append(options, kratosgrpc.TLSConfig(tlsConfig))
	}
	server := kratosgrpc.NewServer(options...)
	accesslogservice.RegisterAccessLogServiceServer(server, service)
	return server, nil
}
