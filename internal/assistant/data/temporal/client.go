// Package temporal 提供运维助手的 Temporal 客户端。
package temporal

import (
	"context"
	"fmt"
	"log/slog"

	temporalclient "go.temporal.io/sdk/client"
	temporallog "go.temporal.io/sdk/log"

	"github.com/lgc202/ingate/internal/assistant/conf"
)

// Client 同时向 Worker 提供 Temporal SDK 能力，并向业务层提供健康检查。
type Client struct {
	temporalclient.Client
}

// New 在配置的启动期限内连接 Temporal。
func New(ctx context.Context, config *conf.Temporal, logger *slog.Logger) (*Client, error) {
	connectCtx, cancel := context.WithTimeout(ctx, config.GetConnectTimeout().AsDuration())
	defer cancel()

	client, err := temporalclient.DialContext(connectCtx, temporalclient.Options{
		HostPort:  config.GetAddress(),
		Namespace: config.GetNamespace(),
		Logger:    temporallog.NewStructuredLogger(logger.With("component", "temporal")),
	})
	if err != nil {
		return nil, fmt.Errorf("connect Temporal: %w", err)
	}
	return &Client{Client: client}, nil
}

// Check 确认 Temporal Frontend 能在调用方期限内响应。
func (c *Client) Check(ctx context.Context) error {
	_, err := c.CheckHealth(ctx, &temporalclient.CheckHealthRequest{})
	return err
}
