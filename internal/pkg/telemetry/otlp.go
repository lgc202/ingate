package telemetry

import (
	"context"
	"crypto/tls"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func newOTLPExporter(ctx context.Context, config TraceConfig) (sdktrace.SpanExporter, error) {
	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.Endpoint),
		otlptracegrpc.WithTimeout(config.ExportTimeout),
		// Collector 地址由配置明确给出，不需要 DNS TXT 下发 gRPC service config。
		// 禁用该查询可避免 Docker 内置 DNS 不响应 TXT 时连带阻塞正常的 A 记录连接。
		otlptracegrpc.WithDialOption(grpc.WithDisableServiceConfig()),
	}
	if config.Insecure {
		options = append(options, otlptracegrpc.WithInsecure())
	} else {
		tlsConfig := config.TLS
		if tlsConfig == nil {
			tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		options = append(options, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig)))
	}
	return otlptracegrpc.New(ctx, options...)
}
