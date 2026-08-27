// Package data 装配运维助手的数据访问与模型实现。
package data

import (
	"context"
	"log/slog"

	"github.com/google/wire"

	agentbiz "github.com/lgc202/ingate/internal/assistant/biz/agent"
	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	executionbiz "github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/biz/modelconfig"
	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/assistant/data/adminapi"
	"github.com/lgc202/ingate/internal/assistant/data/chatmodel"
	"github.com/lgc202/ingate/internal/assistant/data/mysql"
	redisdata "github.com/lgc202/ingate/internal/assistant/data/redis"
)

// ProviderSet 将持久化、事件流、资源查询和模型 SDK 绑定到调用方声明的依赖边界。
var ProviderSet = wire.NewSet(
	NewMySQLStore,
	NewEventStore,
	NewAdminClient,
	NewChatModelFactory,
	wire.Bind(new(agenttool.QuerySource), new(*adminapi.Client)),
	wire.Bind(new(conversation.Store), new(*mysql.Store)),
	wire.Bind(new(executionbiz.ServiceStore), new(*mysql.Store)),
	wire.Bind(new(executionbiz.ExecutorStore), new(*mysql.Store)),
	wire.Bind(new(executionbiz.EventStore), new(*redisdata.EventStore)),
	wire.Bind(new(modelconfig.Store), new(*mysql.Store)),
)

// NewChatModelFactory 把具体模型 SDK 适配器提供给 Agent 声明的创建边界。
func NewChatModelFactory() agentbiz.ChatModelFactory {
	return chatmodel.New
}

// NewAdminClient 创建 Assistant 使用的内部 Admin API 客户端，并由 Wire 统一释放连接。
func NewAdminClient(
	ctx context.Context,
	config *conf.AdminAPI,
	logger *slog.Logger,
) (*adminapi.Client, func(), error) {
	client, err := adminapi.New(ctx, adminapi.Config{
		Address: config.GetAddr(),
		Timeout: config.GetTimeout().AsDuration(),
	})
	if err != nil {
		return nil, nil, err
	}
	return client, func() {
		if err := client.Close(); err != nil {
			logger.Error("close Admin API gRPC client failed", "err", err)
		}
	}, nil
}

// NewMySQLStore 创建持久存储，并把连接池释放交给 Wire cleanup。
func NewMySQLStore(
	ctx context.Context,
	config *conf.Data_MySQL,
	logger *slog.Logger,
) (*mysql.Store, func(), error) {
	store, err := mysql.NewStore(ctx, config)
	if err != nil {
		return nil, nil, err
	}
	return store, func() {
		if err := store.Close(); err != nil {
			logger.Error("close MySQL failed", "err", err)
		}
	}, nil
}

// NewEventStore 创建短期事件存储，并把 Redis 客户端释放交给 Wire cleanup。
func NewEventStore(
	ctx context.Context,
	config *conf.Data_Redis,
	stream *conf.Stream,
	logger *slog.Logger,
) (*redisdata.EventStore, func(), error) {
	store, err := redisdata.NewEventStore(ctx, config, stream)
	if err != nil {
		return nil, nil, err
	}
	return store, func() {
		if err := store.Close(); err != nil {
			logger.Error("close Redis failed", "err", err)
		}
	}, nil
}
