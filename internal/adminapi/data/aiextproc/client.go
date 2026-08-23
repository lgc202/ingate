// Package aiextproc 实现 Admin API 到 AI ExtProc 实时额度查询的 gRPC 适配
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
)

// Client 保存 AI ExtProc 内部查询客户端
type Client struct {
	usage aiextprocv1.TokenQuotaUsageServiceClient
}

// NewClient 创建 Admin API 访问 AI ExtProc 的内部 gRPC 客户端
func NewClient(config *conf.Data, logger *slog.Logger) (*Client, func(), error) {
	settings := config.GetAiExtProc()
	connection, err := kratosgrpc.NewClient(
		context.Background(),
		kratosgrpc.WithEndpoint("dns:///"+settings.GetAddr()),
		kratosgrpc.WithTimeout(settings.GetTimeout().AsDuration()),
		kratosgrpc.WithOptions(googlegrpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create AI ExtProc gRPC client: %w", err)
	}
	cleanup := func() {
		if err := connection.Close(); err != nil {
			logger.Error("close AI ExtProc gRPC client failed", "err", err)
		}
	}
	return &Client{usage: aiextprocv1.NewTokenQuotaUsageServiceClient(connection)}, cleanup, nil
}
