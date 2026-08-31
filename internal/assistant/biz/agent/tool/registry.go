package tool

import (
	"context"

	einotool "github.com/cloudwego/eino/components/tool"

	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
)

const (
	listGatewaysTool        = "list_gateways"
	listRoutesTool          = "list_routes"
	listServicesTool        = "list_services"
	getRouteConfigTool      = "get_route_configuration"
	analyzeTrafficTool      = "analyze_traffic"
	listRecentFailuresTool  = "list_recent_failures"
	getRequestRecordTool    = "get_request_record"
	getCallerTokenQuotaTool = "get_caller_token_quota"
	createGatewayTool       = "create_gateway"
	createServiceTool       = "create_service"
	defaultListLimit        = 20
	maxListLimit            = 50
)

type listResourcesInput struct {
	Query string `json:"query,omitempty"`
	Limit int32  `json:"limit,omitempty"`
}

// QuerySource 组合当前工具注册表要求的外部查询能力。
// 每个具体工具仍只依赖与自身相邻定义的窄接口。
type QuerySource interface {
	GatewayReader
	RouteReader
	RouteConfigurationReader
	ServiceReader
	TrafficReader
	FailureReader
	RequestRecordReader
	CallerTokenQuotaReader
}

// ChangeWriter 是有副作用工具实际调用的 Admin API 边界。
// 工具只有在 Eino 恢复数据明确批准当前中断后才会调用这些方法。
type ChangeWriter interface {
	CreateGateway(context.Context, changebiz.CreateGateway) (changebiz.CreatedResource, error)
	CreateService(context.Context, changebiz.CreateService) (changebiz.CreatedResource, error)
}

// NewTools 创建运维 Agent 当前可以提供给模型的查询和配置变更工具。
// 返回顺序保持稳定，便于观察不同版本暴露给模型的能力变化。
func NewTools(source QuerySource, writer ChangeWriter) ([]einotool.BaseTool, error) {
	gateways, err := newGatewayTool(source)
	if err != nil {
		return nil, err
	}
	routes, err := newRouteTool(source)
	if err != nil {
		return nil, err
	}
	services, err := newServiceTool(source)
	if err != nil {
		return nil, err
	}
	routeConfiguration, err := newRouteConfigurationTool(source)
	if err != nil {
		return nil, err
	}
	traffic, err := newTrafficTool(source)
	if err != nil {
		return nil, err
	}
	failures, err := newFailureTool(source)
	if err != nil {
		return nil, err
	}
	requestRecord, err := newRequestRecordTool(source)
	if err != nil {
		return nil, err
	}
	callerTokenQuota, err := newCallerTokenQuotaTool(source)
	if err != nil {
		return nil, err
	}
	createGateway, err := newCreateGatewayTool(writer)
	if err != nil {
		return nil, err
	}
	createService, err := newCreateServiceTool(writer)
	if err != nil {
		return nil, err
	}
	return []einotool.BaseTool{
		gateways,
		routes,
		services,
		routeConfiguration,
		traffic,
		failures,
		requestRecord,
		callerTokenQuota,
		createGateway,
		createService,
	}, nil
}

func listLimit(limit int32) int32 {
	if limit <= 0 {
		return defaultListLimit
	}
	return min(limit, maxListLimit)
}

func resultStatus(hasMore bool) string {
	if hasMore {
		return "partial"
	}
	return "complete"
}
