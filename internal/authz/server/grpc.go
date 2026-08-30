package server

import (
	"fmt"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	"github.com/lgc202/ingate/internal/authz/conf"
	authzservice "github.com/lgc202/ingate/internal/authz/service"
	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

// NewGRPCServer 创建并注册 Envoy External Authorization 服务。
func NewGRPCServer(
	config *conf.Server,
	service *authzservice.AuthorizationService,
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
		return nil, fmt.Errorf("configure Authz gRPC TLS: %w", err)
	}
	if tlsConfig != nil {
		options = append(options, kratosgrpc.TLSConfig(tlsConfig))
	}

	server := kratosgrpc.NewServer(options...)
	authv3.RegisterAuthorizationServer(server, service)
	return server, nil
}
