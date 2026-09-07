package telemetry

import (
	"context"
	"crypto/tls"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials"
)

func newOTLPExporter(ctx context.Context, config TraceConfig) (sdktrace.SpanExporter, error) {
	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.Endpoint),
		otlptracegrpc.WithTimeout(config.ExportTimeout),
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
