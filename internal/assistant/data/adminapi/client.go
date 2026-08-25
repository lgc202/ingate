// Package adminapi 提供 Assistant 访问内部 Admin API 的 gRPC 客户端。
package adminapi

import (
	"context"
	"fmt"
	"time"

	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
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
	}, nil
}

// Close 释放底层 gRPC 连接。
func (c *Client) Close() error {
	return c.connection.Close()
}

// ListGateways 查询当前配置域中的网关入口。
func (c *Client) ListGateways(
	ctx context.Context,
	query string,
	limit int32,
) (*adminv1.ListGatewaysResponse, error) {
	result, err := c.gateways.ListGateways(ctx, &adminv1.ListGatewaysRequest{Query: query, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("list gateways from Admin API: %w", err)
	}
	return result, nil
}

// ListRoutes 查询当前配置域中的路由。
func (c *Client) ListRoutes(
	ctx context.Context,
	query string,
	limit int32,
) (*adminv1.ListRoutesResponse, error) {
	result, err := c.routes.ListRoutes(ctx, &adminv1.ListRoutesRequest{Query: query, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("list routes from Admin API: %w", err)
	}
	return result, nil
}

// ListServices 查询当前配置域中的普通服务和模型服务。
func (c *Client) ListServices(
	ctx context.Context,
	query string,
	limit int32,
) (*adminv1.ListUpstreamsResponse, error) {
	result, err := c.services.ListUpstreams(ctx, &adminv1.ListUpstreamsRequest{Query: query, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("list services from Admin API: %w", err)
	}
	return result, nil
}

// GetTrafficAnalysis 查询指定时间和资源范围内的聚合流量信号。
func (c *Client) GetTrafficAnalysis(
	ctx context.Context,
	request *adminv1.GetTrafficAnalysisRequest,
) (*adminv1.GetTrafficAnalysisResponse, error) {
	result, err := c.traffic.GetTrafficAnalysis(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("get traffic analysis from Admin API: %w", err)
	}
	return result, nil
}

// ListRequestRecords 查询排障所需的请求元数据，不读取请求内容和凭据。
func (c *Client) ListRequestRecords(
	ctx context.Context,
	request *adminv1.ListRequestRecordsRequest,
) (*adminv1.ListRequestRecordsResponse, error) {
	result, err := c.records.ListRequestRecords(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("list request records from Admin API: %w", err)
	}
	return result, nil
}
