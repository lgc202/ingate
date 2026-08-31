// Package analytics 实现 Admin API 到 Analytics 查询服务的 gRPC 适配。
package analytics

import (
	"context"
	"fmt"
	"log/slog"

	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

// NewClient 创建 Admin API 访问 Analytics 的 gRPC 连接。
func NewClient(
	ctx context.Context,
	config *conf.Data,
	logger *slog.Logger,
) (*googlegrpc.ClientConn, func(), error) {
	settings := config.GetAnalytics()
	tlsSettings := settings.GetTls()
	tlsConfig, err := tlsconfig.NewClient(tlsconfig.ClientConfig{
		Enabled:         tlsSettings.GetEnabled(),
		CAFile:          tlsSettings.GetCaFile(),
		CertificateFile: tlsSettings.GetCertFile(),
		PrivateKeyFile:  tlsSettings.GetKeyFile(),
		ServerName:      tlsSettings.GetServerName(),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("configure Analytics gRPC TLS: %w", err)
	}
	options := []kratosgrpc.ClientOption{
		kratosgrpc.WithEndpoint("dns:///" + settings.GetAddr()),
		kratosgrpc.WithTimeout(settings.GetTimeout().AsDuration()),
	}
	if tlsConfig != nil {
		options = append(options, kratosgrpc.WithTLSConfig(tlsConfig))
	} else {
		options = append(options, kratosgrpc.WithOptions(
			googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
		))
	}
	connection, err := kratosgrpc.NewClient(ctx, options...)
	if err != nil {
		return nil, nil, fmt.Errorf("create Analytics gRPC client: %w", err)
	}
	cleanup := func() {
		if err := connection.Close(); err != nil {
			logger.Error("close Analytics gRPC client failed", "err", err)
		}
	}
	return connection, cleanup, nil
}

func isUnavailable(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}
