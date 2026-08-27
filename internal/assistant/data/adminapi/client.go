// Package adminapi 提供 Assistant 访问内部 Admin API 的 gRPC 客户端。
package adminapi

import (
	"context"
	"fmt"
	"time"

	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	"golang.org/x/sync/errgroup"
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
		gateways = append(gateways, gatewayFromAPI(gateway))
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
		routes = append(routes, routeFromAPI(route))
	}
	return agenttool.RoutePage{
		Items:   routes,
		HasMore: result.GetNextCursor() != "",
	}, nil
}

// GetRouteConfiguration 读取一条路由及其直接引用的网关和服务。
// 这里按引用 ID 精确查询，既不会扫描全部资源，也不会把 Admin API 客户端暴露给 Agent 业务层。
func (c *Client) GetRouteConfiguration(
	ctx context.Context,
	routeID string,
) (agenttool.RouteConfiguration, error) {
	route, err := c.routes.GetRoute(ctx, &adminv1.GetRouteRequest{Id: routeID})
	if err != nil {
		return agenttool.RouteConfiguration{}, queryTargetError(
			fmt.Sprintf("get route %s from Admin API", routeID),
			err,
		)
	}

	gateways := make([]agenttool.Gateway, 0, len(route.GetGatewayIds()))
	for _, gatewayID := range route.GetGatewayIds() {
		gateway, err := c.gateways.GetGateway(ctx, &adminv1.GetGatewayRequest{Id: gatewayID})
		if err != nil {
			return agenttool.RouteConfiguration{}, queryTargetError(
				fmt.Sprintf("get gateway %s referenced by route %s", gatewayID, routeID),
				err,
			)
		}
		gateways = append(gateways, gatewayFromAPI(gateway))
	}

	serviceIDs := routeServiceIDs(route)
	services := make([]agenttool.Service, 0, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		service, err := c.services.GetUpstream(ctx, &adminv1.GetUpstreamRequest{Id: serviceID})
		if err != nil {
			return agenttool.RouteConfiguration{}, queryTargetError(
				fmt.Sprintf("get service %s referenced by route %s", serviceID, routeID),
				err,
			)
		}
		services = append(services, serviceFromAPI(service))
	}

	return agenttool.RouteConfiguration{
		Route:             routeFromAPI(route),
		Hostnames:         route.GetHostnames(),
		PathMatchType:     routePathMatchType(route.GetMatch().GetPath().GetType()),
		Methods:           routeMethods(route.GetMatch().GetMethods()),
		Targets:           routeTargets(route),
		RequestTimeout:    milliseconds(route.GetTimeout().GetRequestMillis()),
		RetryAttempts:     route.GetRetry().GetAttempts(),
		PerTryTimeout:     milliseconds(route.GetRetry().GetPerTryTimeoutMillis()),
		HostRewriteMode:   hostRewriteMode(route.GetHostRewrite().GetMode()),
		HostRewriteTarget: route.GetHostRewrite().GetHostname(),
		Gateways:          gateways,
		Services:          services,
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
		services = append(services, serviceFromAPI(service))
	}
	return agenttool.ServicePage{
		Items:   services,
		HasMore: result.GetNextCursor() != "",
	}, nil
}

func gatewayFromAPI(gateway *adminv1.Gateway) agenttool.Gateway {
	listeners := make([]agenttool.Listener, 0, len(gateway.GetListeners()))
	for _, listener := range gateway.GetListeners() {
		listeners = append(listeners, agenttool.Listener{
			Name:     listener.GetName(),
			Protocol: gatewayProtocol(listener.GetProtocol()),
			Port:     listener.GetPort(),
			Hostname: listener.GetHostname(),
		})
	}
	return agenttool.Gateway{
		ID:        gateway.GetId(),
		Name:      gateway.GetName(),
		Enabled:   gateway.GetEnabled(),
		State:     resourceState(gateway.GetState()),
		Message:   gateway.GetMessage(),
		Listeners: listeners,
	}
}

func routeFromAPI(route *adminv1.Route) agenttool.Route {
	return agenttool.Route{
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
	}
}

func serviceFromAPI(service *adminv1.Upstream) agenttool.Service {
	return agenttool.Service{
		ID:            service.GetId(),
		Name:          service.GetName(),
		Type:          serviceType(service),
		State:         resourceState(service.GetState()),
		Message:       service.GetMessage(),
		EndpointCount: len(service.GetEndpoints()),
		TLS:           service.GetTls() != nil,
		ModelProtocol: modelProtocol(service.GetModel().GetProtocol()),
	}
}

// AnalyzeTraffic 查询指定时间和资源范围内的汇总指标与资源排名。
func (c *Client) AnalyzeTraffic(
	ctx context.Context,
	query agenttool.TrafficQuery,
) (agenttool.TrafficAnalysis, error) {
	request := &adminv1.GetTrafficAnalysisRequest{
		StartTime:          timestamppb.New(query.StartTime),
		EndTime:            timestamppb.New(query.EndTime),
		BreakdownDimension: trafficDimension(query.GroupBy),
		BreakdownLimit:     query.Limit,
		BreakdownOrder:     trafficOrder(query.OrderBy),
	}
	applyTrafficScope(request, query.ScopeType, query.ScopeID)
	result, err := c.traffic.GetTrafficAnalysis(ctx, request)
	if err != nil {
		return agenttool.TrafficAnalysis{}, fmt.Errorf("get traffic analysis from Admin API: %w", err)
	}

	dimension := trafficDimensionFromAPI(result.GetBreakdownDimension())
	breakdown := result.GetBreakdown()
	items := make([]agenttool.ResourceTrafficMetrics, len(breakdown))
	group, lookupCtx := errgroup.WithContext(ctx)
	for index, item := range breakdown {
		group.Go(func() error {
			name, err := c.resourceName(lookupCtx, dimension, item.GetResourceId())
			if err != nil {
				return err
			}
			items[index] = agenttool.ResourceTrafficMetrics{
				ID:      item.GetResourceId(),
				Name:    name,
				Metrics: trafficMetrics(item.GetMetrics()),
			}
			return nil
		})
	}
	// 排名已经限制返回数量，各资源名称之间没有依赖。并行精确查询既保持排名顺序，
	// 也避免按条目数线性叠加 Admin API 往返时间。
	if err := group.Wait(); err != nil {
		return agenttool.TrafficAnalysis{}, err
	}

	return agenttool.TrafficAnalysis{
		Summary: trafficMetrics(result.GetSummary()),
		GroupBy: dimension,
		OrderBy: trafficOrderFromAPI(result.GetBreakdownOrder()),
		Items:   items,
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
	applyFailureScope(request, query.ScopeType, query.ScopeID)
	result, err := c.records.ListRequestRecords(ctx, request)
	if err != nil {
		return agenttool.FailurePage{}, fmt.Errorf("list request records from Admin API: %w", err)
	}

	records := make([]agenttool.Failure, 0, len(result.GetRecords()))
	for _, record := range result.GetRecords() {
		records = append(records, agenttool.Failure{
			RecordID:   record.GetId(),
			StartedAt:  protoTime(record.GetStartedAt()),
			Method:     record.GetMethod(),
			Host:       record.GetHost(),
			Path:       record.GetPath(),
			StatusCode: record.GetStatusCode(),
			Duration:   protoDuration(record.GetDuration()),
			GatewayID:  record.GetGatewayId(),
			RouteID:    record.GetRouteId(),
			ServiceID:  record.GetServiceId(),
		})
	}
	scopeName := ""
	if query.ScopeType != "all" {
		scopeName, err = c.resourceName(ctx, agenttool.TrafficDimension(query.ScopeType), query.ScopeID)
		if err != nil {
			return agenttool.FailurePage{}, err
		}
	}
	return agenttool.FailurePage{
		ScopeName: scopeName,
		Items:     records,
		HasMore:   result.GetNextPageToken() != "",
	}, nil
}

// GetRequestRecord 按列表返回的记录标识和开始时间读取单次请求元数据。
// startedAt 同时作为 ClickHouse 分区查询条件，避免为一条记录扫描全部保留数据。
func (c *Client) GetRequestRecord(
	ctx context.Context,
	recordID string,
	startedAt time.Time,
) (agenttool.RequestRecord, error) {
	result, err := c.records.GetRequestRecord(ctx, &adminv1.GetRequestRecordRequest{
		Id:        recordID,
		StartedAt: timestamppb.New(startedAt),
	})
	if err != nil {
		return agenttool.RequestRecord{}, queryTargetError(
			fmt.Sprintf("get request record %s from Admin API", recordID),
			err,
		)
	}

	return requestRecordFromAPI(result), nil
}

// GetCallerTokenQuota 查询调用方身份和当前实际执行的额度用量。
// 两个 Admin API 结果在此处组合，Agent 业务层无需了解调用方与策略服务的协议边界。
func (c *Client) GetCallerTokenQuota(
	ctx context.Context,
	callerID string,
) (agenttool.CallerTokenQuota, error) {
	caller, err := c.callers.GetCaller(ctx, &adminv1.GetCallerRequest{Id: callerID})
	if err != nil {
		return agenttool.CallerTokenQuota{}, queryTargetError(
			fmt.Sprintf("get caller %s from Admin API", callerID),
			err,
		)
	}

	result, err := c.tokenQuota.GetCallerTokenQuotaUsage(
		ctx,
		&adminv1.GetCallerTokenQuotaUsageRequest{CallerId: callerID},
	)
	if err != nil {
		return agenttool.CallerTokenQuota{}, queryTargetError(
			fmt.Sprintf("get caller %s token quota usage from Admin API", callerID),
			err,
		)
	}

	usages := make([]agenttool.TokenQuotaUsage, 0, len(result.GetUsages()))
	for _, usage := range result.GetUsages() {
		usages = append(usages, tokenQuotaUsageFromAPI(usage))
	}
	return agenttool.CallerTokenQuota{
		CallerID:   caller.GetId(),
		CallerName: caller.GetName(),
		Enabled:    caller.GetEnabled(),
		Usages:     usages,
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

func routePathMatchType(matchType adminv1.RoutePathMatchType) string {
	switch matchType {
	case adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_EXACT:
		return "exact"
	default:
		return "prefix"
	}
}

func routeMethods(methods []adminv1.HTTPMethod) []string {
	result := make([]string, 0, len(methods))
	for _, method := range methods {
		if name := httpMethod(method); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func httpMethod(method adminv1.HTTPMethod) string {
	switch method {
	case adminv1.HTTPMethod_HTTP_METHOD_GET:
		return "GET"
	case adminv1.HTTPMethod_HTTP_METHOD_HEAD:
		return "HEAD"
	case adminv1.HTTPMethod_HTTP_METHOD_POST:
		return "POST"
	case adminv1.HTTPMethod_HTTP_METHOD_PUT:
		return "PUT"
	case adminv1.HTTPMethod_HTTP_METHOD_PATCH:
		return "PATCH"
	case adminv1.HTTPMethod_HTTP_METHOD_DELETE:
		return "DELETE"
	case adminv1.HTTPMethod_HTTP_METHOD_OPTIONS:
		return "OPTIONS"
	default:
		return ""
	}
}

// routeTargets 同时保留客户端模型名和厂商模型名，便于判断 AI 路由是否选错线路。
// 普通 API 路由没有模型映射，只返回服务和权重。
func routeTargets(route *adminv1.Route) []agenttool.RouteTarget {
	if route.GetAi() == nil {
		targets := make([]agenttool.RouteTarget, 0, len(route.GetUpstreams()))
		for _, target := range route.GetUpstreams() {
			targets = append(targets, agenttool.RouteTarget{
				ServiceID: target.GetUpstreamId(),
				Weight:    target.GetWeight(),
			})
		}
		return targets
	}

	var count int
	for _, model := range route.GetAi().GetModels() {
		count += len(model.GetTargets())
	}
	targets := make([]agenttool.RouteTarget, 0, count)
	for _, model := range route.GetAi().GetModels() {
		for _, target := range model.GetTargets() {
			targets = append(targets, agenttool.RouteTarget{
				ServiceID:    target.GetUpstreamId(),
				ExposedModel: model.GetName(),
				Model:        target.GetModel(),
				Weight:       target.GetWeight(),
			})
		}
	}
	return targets
}

func hostRewriteMode(mode adminv1.HostRewriteMode) string {
	switch mode {
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_SERVICE_ADDRESS:
		return "service_address"
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_PRESERVE:
		return "preserve"
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_CUSTOM:
		return "custom"
	default:
		return ""
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
func applyTrafficScope(request *adminv1.GetTrafficAnalysisRequest, scopeType, scopeID string) {
	switch scopeType {
	case "gateway":
		request.GatewayId = scopeID
	case "route":
		request.RouteId = scopeID
	case "service":
		request.ServiceId = scopeID
	}
}

func applyFailureScope(request *adminv1.ListRequestRecordsRequest, scopeType, scopeID string) {
	switch scopeType {
	case "gateway":
		request.GatewayId = scopeID
	case "route":
		request.RouteId = scopeID
	case "service":
		request.ServiceId = scopeID
	}
}

func trafficDimension(value agenttool.TrafficDimension) adminv1.TrafficBreakdownDimension {
	switch value {
	case agenttool.TrafficDimensionGateway:
		return adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY
	case agenttool.TrafficDimensionService:
		return adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_SERVICE
	default:
		return adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_ROUTE
	}
}

func trafficDimensionFromAPI(value adminv1.TrafficBreakdownDimension) agenttool.TrafficDimension {
	switch value {
	case adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_GATEWAY:
		return agenttool.TrafficDimensionGateway
	case adminv1.TrafficBreakdownDimension_TRAFFIC_BREAKDOWN_DIMENSION_SERVICE:
		return agenttool.TrafficDimensionService
	default:
		return agenttool.TrafficDimensionRoute
	}
}

func trafficOrder(value agenttool.TrafficOrder) adminv1.TrafficBreakdownOrder {
	switch value {
	case agenttool.TrafficOrderServerErrorRate:
		return adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_SERVER_ERROR_RATE
	case agenttool.TrafficOrderP95Duration:
		return adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_P95_DURATION
	default:
		return adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_REQUEST_COUNT
	}
}

func trafficOrderFromAPI(value adminv1.TrafficBreakdownOrder) agenttool.TrafficOrder {
	switch value {
	case adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_SERVER_ERROR_RATE:
		return agenttool.TrafficOrderServerErrorRate
	case adminv1.TrafficBreakdownOrder_TRAFFIC_BREAKDOWN_ORDER_P95_DURATION:
		return agenttool.TrafficOrderP95Duration
	default:
		return agenttool.TrafficOrderRequestCount
	}
}

// 工具结果必须包含用户可识别的名称，避免模型为解释 UUID 再发起一轮工具调用。
// 已删除资源的历史流量仍可能出现在排名中，此时保留 ID 作为可追溯名称。
func (c *Client) resourceName(
	ctx context.Context,
	dimension agenttool.TrafficDimension,
	resourceID string,
) (string, error) {
	switch dimension {
	case agenttool.TrafficDimensionGateway:
		gateway, err := c.gateways.GetGateway(ctx, &adminv1.GetGatewayRequest{Id: resourceID})
		return resourceNameResult(dimension, resourceID, gateway.GetName(), err)
	case agenttool.TrafficDimensionService:
		service, err := c.services.GetUpstream(ctx, &adminv1.GetUpstreamRequest{Id: resourceID})
		return resourceNameResult(dimension, resourceID, service.GetName(), err)
	default:
		route, err := c.routes.GetRoute(ctx, &adminv1.GetRouteRequest{Id: resourceID})
		return resourceNameResult(dimension, resourceID, route.GetName(), err)
	}
}

func resourceNameResult(
	dimension agenttool.TrafficDimension,
	resourceID string,
	name string,
	err error,
) (string, error) {
	if status.Code(err) == codes.NotFound {
		return resourceID, nil
	}
	if err != nil {
		return "", fmt.Errorf("get %s %s from Admin API: %w", dimension, resourceID, err)
	}
	if name == "" {
		return resourceID, nil
	}
	return name, nil
}

// queryTargetError 区分已经失效的精确查询目标与 Admin API 故障。
// NotFound 会返回 Agent 循环重新定位资源；其他错误保留原始原因并终止本次执行。
func queryTargetError(operation string, err error) error {
	if status.Code(err) == codes.NotFound {
		return fmt.Errorf("%w: %s: %w", agenttool.ErrQueryTargetNotFound, operation, err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func trafficMetrics(metrics *adminv1.TrafficMetrics) agenttool.TrafficMetrics {
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

func requestRecordFromAPI(record *adminv1.RequestRecord) agenttool.RequestRecord {
	return agenttool.RequestRecord{
		RecordID:         record.GetId(),
		StartedAt:        protoTime(record.GetStartedAt()),
		Duration:         protoDuration(record.GetDuration()),
		TimeToFirstByte:  optionalProtoDuration(record.GetTimeToFirstByte()),
		Method:           record.GetMethod(),
		Host:             record.GetHost(),
		Path:             record.GetPath(),
		StatusCode:       record.GetStatusCode(),
		Outcome:          requestOutcomeFromAPI(record.GetOutcome()),
		RequestBytes:     record.GetRequestBytes(),
		ResponseBytes:    record.GetResponseBytes(),
		GatewayID:        record.GetGatewayId(),
		RouteID:          record.GetRouteId(),
		ServiceID:        record.GetServiceId(),
		Protocol:         record.GetProtocol(),
		RejectionReason:  rejectionReasonFromAPI(record.GetRejectionReason()),
		UpstreamAttempts: record.GetUpstreamAttempts(),
		AIModelCall:      aiModelCallFromAPI(record.GetAiModelCall()),
		CallerID:         record.GetCallerId(),
	}
}

func tokenQuotaUsageFromAPI(usage *adminv1.CallerTokenQuotaUsage) agenttool.TokenQuotaUsage {
	return agenttool.TokenQuotaUsage{
		PolicyID:        usage.GetPolicyId(),
		PolicyName:      usage.GetPolicyName(),
		Period:          tokenQuotaPeriodFromAPI(usage.GetPeriod()),
		UsedTokens:      usage.GetUsedTokens(),
		LimitTokens:     usage.GetLimitTokens(),
		RemainingTokens: usage.GetRemainingTokens(),
		StartedAt:       protoTime(usage.GetStartedAt()),
		ResetsAt:        protoTime(usage.GetResetsAt()),
	}
}

func tokenQuotaPeriodFromAPI(period adminv1.TokenQuotaPeriod) string {
	switch period {
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_DAY:
		return "day"
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_WEEK:
		return "week"
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_MONTH:
		return "month"
	default:
		return "unknown"
	}
}

func requestOutcomeFromAPI(outcome adminv1.RequestOutcome) string {
	switch outcome {
	case adminv1.RequestOutcome_REQUEST_OUTCOME_SUCCESS:
		return "success"
	case adminv1.RequestOutcome_REQUEST_OUTCOME_CLIENT_ERROR:
		return "client_error"
	case adminv1.RequestOutcome_REQUEST_OUTCOME_SERVER_ERROR:
		return "server_error"
	case adminv1.RequestOutcome_REQUEST_OUTCOME_NO_RESPONSE:
		return "no_response"
	default:
		return "unknown"
	}
}

func rejectionReasonFromAPI(reason adminv1.RequestRejectionReason) string {
	switch reason {
	case adminv1.RequestRejectionReason_REQUEST_REJECTION_REASON_TOKEN_QUOTA_EXCEEDED:
		return "token_quota_exceeded"
	default:
		return ""
	}
}

func aiModelCallFromAPI(call *adminv1.AIModelCall) *agenttool.AIModelCall {
	if call == nil {
		return nil
	}
	return &agenttool.AIModelCall{
		ClientModel:   call.GetClientModel(),
		UpstreamModel: call.GetUpstreamModel(),
		Protocol:      modelProtocol(call.GetProtocol()),
		ResponseModel: call.GetResponseModel(),
		FinishReason:  call.GetFinishReason(),
		InputTokens:   copyOptionalUint64(call.InputTokens),
		OutputTokens:  copyOptionalUint64(call.OutputTokens),
		TotalTokens:   copyOptionalUint64(call.TotalTokens),
	}
}

func copyOptionalUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
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
