// Package data 装配运维助手的外部依赖。
package data

import (
	"context"
	"log/slog"

	"github.com/google/wire"
	temporalclient "go.temporal.io/sdk/client"

	"github.com/lgc202/ingate/internal/assistant/biz/system"
	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/assistant/data/modelendpoint"
	"github.com/lgc202/ingate/internal/assistant/data/mysql"
	temporaldata "github.com/lgc202/ingate/internal/assistant/data/temporal"
)

// ProviderSet 提供 MySQL、Temporal 和模型健康端点适配器。
var ProviderSet = wire.NewSet(
	NewDatabase,
	NewTemporalClient,
	NewTemporalSDKClient,
	modelendpoint.New,
	wire.Bind(new(system.DatabaseChecker), new(*mysql.Database)),
	wire.Bind(new(system.WorkflowChecker), new(*temporaldata.Client)),
	wire.Bind(new(system.ModelChecker), new(*modelendpoint.Endpoint)),
)

// NewDatabase 创建 MySQL 连接池，并把释放动作交给 Wire cleanup。
func NewDatabase(config *conf.Data_MySQL, logger *slog.Logger) (*mysql.Database, func(), error) {
	database, err := mysql.New(config)
	if err != nil {
		return nil, nil, err
	}
	return database, func() {
		if err := database.Close(); err != nil {
			logger.Error("close MySQL failed", "err", err)
		}
	}, nil
}

// NewTemporalClient 创建 Temporal 客户端，并把释放动作交给 Wire cleanup。
func NewTemporalClient(
	ctx context.Context,
	config *conf.Temporal,
	logger *slog.Logger,
) (*temporaldata.Client, func(), error) {
	client, err := temporaldata.New(ctx, config, logger)
	if err != nil {
		return nil, nil, err
	}
	return client, client.Close, nil
}

// NewTemporalSDKClient 向 Worker 提供底层 Temporal SDK 客户端。
func NewTemporalSDKClient(client *temporaldata.Client) temporalclient.Client {
	return client.Client
}
