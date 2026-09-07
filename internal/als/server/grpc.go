package server

import (
	"fmt"

	accesslogservice "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	"github.com/lgc202/ingate/internal/als/conf"
	alsservice "github.com/lgc202/ingate/internal/als/service"
	"github.com/lgc202/ingate/internal/pkg/telemetry"
	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

// NewGRPCServer 创建并注册 Envoy ALS gRPC 服务。
func NewGRPCServer(
	config *conf.Server,
	service *alsservice.Service,
	tracing *telemetry.Tracing,
) (*kratosgrpc.Server, error) {
	traceHandler := otelgrpc.NewServerHandler(
		otelgrpc.WithTracerProvider(tracing.Provider()),
		otelgrpc.WithPropagators(tracing.Propagator()),
	)
	options := []kratosgrpc.ServerOption{
		kratosgrpc.Network("tcp"),
		kratosgrpc.Address(config.GetGrpc().GetAddr()),
		// gRPC StatsHandler 覆盖完整流生命周期，并把 Span Context 传入 ALS 业务处理。
		kratosgrpc.Options(grpc.StatsHandler(traceHandler)),
	}
	tlsSettings := config.GetGrpc().GetTls()
	tlsConfig, err := tlsconfig.NewServer(tlsconfig.ServerConfig{
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
