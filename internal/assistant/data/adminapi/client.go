// Package adminapi 提供 Assistant 访问内部 Admin API 的 gRPC 客户端。
package adminapi

import (
	"context"
	"fmt"
	"time"

	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
func (c *Client) ListGateways(ctx context.Context, query agenttool.ResourceListQuery) (agenttool.GatewayPage, error) {
	result, err := c.gateways.ListGateways(ctx, &adminv1.ListGatewaysRequest{
		Query: query.Text,
		Limit: query.Limit,
	})
	if err != nil {
		return agenttool.GatewayPage{}, fmt.Errorf("list gateways from Admin API: %w", err)
	}

	gateways := make([]agenttool.Gateway, 0, len(result.GetGateways()))
	for _, gateway := range result.GetGateways() {
		listeners := make([]agenttool.Listener, 0, len(gateway.GetListeners()))
		for _, listener := range gateway.GetListeners() {
			listeners = append(listeners, agenttool.Listener{
				Name:     listener.GetName(),
				Protocol: gatewayProtocol(listener.GetProtocol()),
				Port:     listener.GetPort(),
				Hostname: listener.GetHostname(),
			})
		}
		gateways = append(gateways, agenttool.Gateway{
			ID:        gateway.GetId(),
			Name:      gateway.GetName(),
			Enabled:   gateway.GetEnabled(),
			State:     resourceState(gateway.GetState()),
			Message:   gateway.GetMessage(),
			Listeners: listeners,
		})
	}
	return agenttool.GatewayPage{
		Items:   gateways,
		HasMore: result.GetNextCursor() != "",
	}, nil
}

// ListRoutes 查询当前配置域中的路由。
func (c *Client) ListRoutes(ctx context.Context, query agenttool.ResourceListQuery) (agenttool.RoutePage, error) {
	result, err := c.routes.ListRoutes(ctx, &adminv1.ListRoutesRequest{
		Query: query.Text,
		Limit: query.Limit,
	})
	if err != nil {
		return agenttool.RoutePage{}, fmt.Errorf("list routes from Admin API: %w", err)
	}

	routes := make([]agenttool.Route, 0, len(result.GetRoutes()))
	for _, route := range result.GetRoutes() {
		routes = append(routes, agenttool.Route{
			ID:            route.GetId(),
			Name:          route.GetName(),
			Type:          routeType(route),
			Enabled:       route.GetEnabled(),
			State:         resourceState(route.GetState()),
			Message:       route.GetMessage(),
			AccessMode:    routeAccessMode(route.GetAccessMode()),
			GatewayIDs:    route.GetGatewayIds(),
			Path:          route.GetMatch().GetPath().GetValue(),
			ServiceIDs:    routeServiceIDs(route),
			ExposedModels: routeModelNames(route),
		})
	}
	return agenttool.RoutePage{
		Items:   routes,
		HasMore: result.GetNextCursor() != "",
	}, nil
}

// ListServices 查询当前配置域中的普通服务和模型服务。
func (c *Client) ListServices(ctx context.Context, query agenttool.ResourceListQuery) (agenttool.ServicePage, error) {
	result, err := c.services.ListUpstreams(ctx, &adminv1.ListUpstreamsRequest{
		Query: query.Text,
		Limit: query.Limit,
	})
	if err != nil {
		return agenttool.ServicePage{}, fmt.Errorf("list services from Admin API: %w", err)
	}

	services := make([]agenttool.Service, 0, len(result.GetUpstreams()))
	for _, service := range result.GetUpstreams() {
		services = append(services, agenttool.Service{
			ID:            service.GetId(),
			Name:          service.GetName(),
			Type:          serviceType(service),
			State:         resourceState(service.GetState()),
			Message:       service.GetMessage(),
			EndpointCount: len(service.GetEndpoints()),
			TLS:           service.GetTls() != nil,
			ModelProtocol: modelProtocol(service.GetModel().GetProtocol()),
		})
	}
	return agenttool.ServicePage{
		Items:   services,
		HasMore: result.GetNextCursor() != "",
	}, nil
}

// GetTraffic 查询指定时间和资源范围内的聚合流量信号。
func (c *Client) GetTraffic(ctx context.Context, query agenttool.TrafficQuery) (agenttool.TrafficMetrics, error) {
	request := &adminv1.GetTrafficAnalysisRequest{
		StartTime: timestamppb.New(query.StartTime),
		EndTime:   timestamppb.New(query.EndTime),
	}
	applyTrafficScope(request, query.ResourceType, query.ResourceID)
	result, err := c.traffic.GetTrafficAnalysis(ctx, request)
	if err != nil {
		return agenttool.TrafficMetrics{}, fmt.Errorf("get traffic analysis from Admin API: %w", err)
	}

	metrics := result.GetSummary()
	return agenttool.TrafficMetrics{
		RequestCount:     metrics.GetRequestCount(),
		NonErrorCount:    metrics.GetNonErrorCount(),
		ClientErrorCount: metrics.GetClientErrorCount(),
		ServerErrorCount: metrics.GetServerErrorCount(),
		NoResponseCount:  metrics.GetNoResponseCount(),
		AverageDuration:  protoDuration(metrics.GetAverageDuration()),
		P50Duration:      protoDuration(metrics.GetP50Duration()),
		P95Duration:      protoDuration(metrics.GetP95Duration()),
		P99Duration:      protoDuration(metrics.GetP99Duration()),
	}, nil
}

// ListFailures 查询排障所需的失败请求元数据，不读取请求内容和凭据。
func (c *Client) ListFailures(ctx context.Context, query agenttool.FailureQuery) (agenttool.FailurePage, error) {
	request := &adminv1.ListRequestRecordsRequest{
		StartTime: timestamppb.New(query.StartTime),
		EndTime:   timestamppb.New(query.EndTime),
		Outcome:   requestOutcome(query.Outcome),
		PageSize:  query.Limit,
	}
	applyFailureScope(request, query.ResourceType, query.ResourceID)
	result, err := c.records.ListRequestRecords(ctx, request)
	if err != nil {
		return agenttool.FailurePage{}, fmt.Errorf("list request records from Admin API: %w", err)
	}

	records := make([]agenttool.Failure, 0, len(result.GetRecords()))
	for _, record := range result.GetRecords() {
		records = append(records, agenttool.Failure{
			StartedAt:  protoTime(record.GetStartedAt()),
			Method:     record.GetMethod(),
			StatusCode: record.GetStatusCode(),
			Duration:   protoDuration(record.GetDuration()),
			GatewayID:  record.GetGatewayId(),
			RouteID:    record.GetRouteId(),
			ServiceID:  record.GetServiceId(),
		})
	}
	return agenttool.FailurePage{
		Items:   records,
		HasMore: result.GetNextPageToken() != "",
	}, nil
}

func resourceState(state adminv1.ResourceState) string {
	switch state {
	case adminv1.ResourceState_DISABLED:
		return "disabled"
	case adminv1.ResourceState_PENDING:
		return "pending"
	case adminv1.ResourceState_READY:
		return "ready"
	case adminv1.ResourceState_ERROR:
		return "error"
	default:
		return "unknown"
	}
}

func gatewayProtocol(protocol adminv1.GatewayProtocol) string {
	switch protocol {
	case adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTP:
		return "http"
	case adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTPS:
		return "https"
	default:
		return "unknown"
	}
}

func routeType(route *adminv1.Route) string {
	if route.GetAi() != nil {
		return "ai"
	}
	return "api"
}

func routeAccessMode(mode adminv1.RouteAccessMode) string {
	switch mode {
	case adminv1.RouteAccessMode_ROUTE_ACCESS_PUBLIC:
		return "public"
	case adminv1.RouteAccessMode_ROUTE_ACCESS_CALLER:
		return "caller"
	default:
		return "unknown"
	}
}

func routeServiceIDs(route *adminv1.Route) []string {
	serviceIDs := make([]string, 0, len(route.GetUpstreams()))
	for _, upstream := range route.GetUpstreams() {
		serviceIDs = appendUnique(serviceIDs, upstream.GetUpstreamId())
	}
	for _, model := range route.GetAi().GetModels() {
		for _, target := range model.GetTargets() {
			serviceIDs = appendUnique(serviceIDs, target.GetUpstreamId())
		}
	}
	return serviceIDs
}

func routeModelNames(route *adminv1.Route) []string {
	models := route.GetAi().GetModels()
	names := make([]string, 0, len(models))
	for _, model := range models {
		names = append(names, model.GetName())
	}
	return names
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func serviceType(service *adminv1.Upstream) string {
	if service.GetModel() != nil {
		return "model"
	}
	return "http"
}

func modelProtocol(protocol adminv1.ModelProtocol) string {
	switch protocol {
	case adminv1.ModelProtocol_MODEL_PROTOCOL_OPENAI:
		return "openai"
	case adminv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC:
		return "anthropic"
	default:
		return ""
	}
}

// 资源范围由工具业务协议表达，只有此处知道对应的 Admin API 字段。
func applyTrafficScope(request *adminv1.GetTrafficAnalysisRequest, resourceType, resourceID string) {
	switch resourceType {
	case "gateway":
		request.GatewayId = resourceID
	case "route":
		request.RouteId = resourceID
	case "service":
		request.ServiceId = resourceID
	}
}

func applyFailureScope(request *adminv1.ListRequestRecordsRequest, resourceType, resourceID string) {
	switch resourceType {
	case "gateway":
		request.GatewayId = resourceID
	case "route":
		request.RouteId = resourceID
	case "service":
		request.ServiceId = resourceID
	}
}

func requestOutcome(outcome agenttool.FailureOutcome) adminv1.RequestOutcome {
	switch outcome {
	case agenttool.FailureOutcomeClientError:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_CLIENT_ERROR
	case agenttool.FailureOutcomeServerError:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_SERVER_ERROR
	case agenttool.FailureOutcomeNoResponse:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_NO_RESPONSE
	default:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_UNSPECIFIED
	}
}

func protoDuration(value *durationpb.Duration) time.Duration {
	if value == nil {
		return 0
	}
	return value.AsDuration()
}

func protoTime(value *timestamppb.Timestamp) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.AsTime()
}
