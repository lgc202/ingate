// Package server 装配 ingate-als 的 Kratos transport 和积压回放任务
package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	accesslogservice "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	"github.com/lgc202/ingate/internal/als/conf"
	alsservice "github.com/lgc202/ingate/internal/als/service"
)

// NewGRPCServer 创建并注册 Envoy ALS gRPC 服务
func NewGRPCServer(config *conf.Server, service *alsservice.Service) (*kratosgrpc.Server, error) {
	options := []kratosgrpc.ServerOption{
		kratosgrpc.Network("tcp"),
		kratosgrpc.Address(config.GetGrpc().GetAddr()),
	}
	tlsConfig, err := serverTLSConfig(config.GetGrpc().GetTls())
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		options = append(options, kratosgrpc.TLSConfig(tlsConfig))
	}
	server := kratosgrpc.NewServer(options...)
	accesslogservice.RegisterAccessLogServiceServer(server, service)
	return server, nil
}

func serverTLSConfig(config *conf.Server_GRPC_TLS) (*tls.Config, error) {
	if config == nil || !config.GetEnabled() {
		return nil, nil
	}
	certificate, err := tls.LoadX509KeyPair(config.GetCertFile(), config.GetKeyFile())
	if err != nil {
		return nil, fmt.Errorf("load ALS server certificate: %w", err)
	}
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
	}
	if config.GetClientCaFile() == "" {
		return tlsConfig, nil
	}

	// 配置 client CA 即启用双向认证，避免远程部署时任意客户端伪造 Envoy 写入记录
	pem, err := os.ReadFile(config.GetClientCaFile())
	if err != nil {
		return nil, fmt.Errorf("read ALS client CA certificate: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("parse ALS client CA certificate")
	}
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.ClientCAs = clientCAs
	return tlsConfig, nil
}
