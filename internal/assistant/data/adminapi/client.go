// Package adminapi 提供 Assistant 访问内部 Admin API 的 gRPC 客户端。
package adminapi

import (
	"context"
	"fmt"
	"time"

	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/google/uuid"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
)

// Config 定义 Assistant 访问 Admin API 的连接参数。
type Config struct {
	Address string
	Timeout time.Duration
}

// Client 复用一条 gRPC 连接，并只暴露当前只读工具实际需要的资源查询。
type Client struct {
	connection *googlegrpc.ClientConn
	gateways   adminv1.GatewayServiceClient
	routes     adminv1.RouteServiceClient
	services   adminv1.UpstreamServiceClient
	traffic    adminv1.TrafficAnalysisServiceClient
	records    adminv1.RequestRecordServiceClient
	callers    adminv1.CallerServiceClient
	tokenQuota adminv1.TokenQuotaPolicyServiceClient
}

// New 创建 Assistant 使用的 Admin API 客户端。
func New(ctx context.Context, config Config) (*Client, error) {
	connection, err := kratosgrpc.NewClient(
		ctx,
		kratosgrpc.WithEndpoint("dns:///"+config.Address),
		kratosgrpc.WithTimeout(config.Timeout),
		kratosgrpc.WithOptions(googlegrpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return nil, fmt.Errorf("create Admin API gRPC client: %w", err)
	}
	return &Client{
		connection: connection,
		gateways:   adminv1.NewGatewayServiceClient(connection),
		routes:     adminv1.NewRouteServiceClient(connection),
		services:   adminv1.NewUpstreamServiceClient(connection),
		traffic:    adminv1.NewTrafficAnalysisServiceClient(connection),
		records:    adminv1.NewRequestRecordServiceClient(connection),
		callers:    adminv1.NewCallerServiceClient(connection),
		tokenQuota: adminv1.NewTokenQuotaPolicyServiceClient(connection),
	}, nil
}

// Close 释放底层 gRPC 连接。
func (c *Client) Close() error {
	return c.connection.Close()
}

// queryTargetError 区分已经失效的精确查询目标与 Admin API 故障。
// NotFound 会返回 Agent 循环重新定位资源；其他错误保留原始原因并终止本次执行。
func queryTargetError(operation string, err error) error {
	if status.Code(err) == codes.NotFound {
		return fmt.Errorf("%w: %s: %w", agenttool.ErrQueryTargetNotFound, operation, err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func optionalProtoDuration(value *durationpb.Duration) *time.Duration {
	if value == nil {
		return nil
	}
	duration := value.AsDuration()
	return &duration
}

func protoDuration(value *durationpb.Duration) time.Duration {
	if value == nil {
		return 0
	}
	return value.AsDuration()
}

func milliseconds(value uint32) time.Duration {
	return time.Duration(value) * time.Millisecond
}

func protoTime(value *timestamppb.Timestamp) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.AsTime()
}

func validTimestamp(value *timestamppb.Timestamp) bool {
	return value != nil && value.CheckValid() == nil
}

func validDuration(value *durationpb.Duration) bool {
	return value != nil && value.CheckValid() == nil && value.AsDuration() >= 0
}

func validResourceID(id string) bool {
	return uuid.Validate(id) == nil
}

func validOptionalResourceID(id string) bool {
	return id == "" || validResourceID(id)
}
