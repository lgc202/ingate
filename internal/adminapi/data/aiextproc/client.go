// Package aiextproc 实现 Admin API 到 AI ExtProc 实时额度查询的 gRPC 适配。
package aiextproc

import (
	"context"
	"fmt"
	"log/slog"

	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	aiextprocv1 "github.com/lgc202/ingate/api/aiextproc/v1"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

// NewClient 创建 Admin API 访问 AI ExtProc 的内部 gRPC 客户端。
func NewClient(
	ctx context.Context,
	config *conf.Data,
	logger *slog.Logger,
) (aiextprocv1.TokenQuotaUsageServiceClient, func(), error) {
	settings := config.GetAiExtProc()
	tlsSettings := settings.GetTls()
	tlsConfig, err := tlsconfig.NewClient(tlsconfig.ClientConfig{
		Enabled:         tlsSettings.GetEnabled(),
		CAFile:          tlsSettings.GetCaFile(),
		CertificateFile: tlsSettings.GetCertFile(),
		PrivateKeyFile:  tlsSettings.GetKeyFile(),
		ServerName:      tlsSettings.GetServerName(),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("configure AI ExtProc gRPC TLS: %w", err)
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
		return nil, nil, fmt.Errorf("create AI ExtProc gRPC client: %w", err)
	}
	cleanup := func() {
		if err := connection.Close(); err != nil {
			logger.Error("close AI ExtProc gRPC client failed", "err", err)
		}
	}
	return aiextprocv1.NewTokenQuotaUsageServiceClient(connection), cleanup, nil
}
