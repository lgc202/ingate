// Package tool 定义模型 Agent 可调用的 Ingate 原子工具。
package tool

import (
	"context"

	einotool "github.com/cloudwego/eino/components/tool"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

const (
	listGatewaysTool       = "list_gateways"
	listRoutesTool         = "list_routes"
	listServicesTool       = "list_services"
	getRecentTrafficTool   = "get_recent_traffic"
	listRecentFailuresTool = "list_recent_failures"
	defaultListLimit       = 20
	maxListLimit           = 50
)

// GatewayReader 是网关列表工具所需的最小查询边界。
type GatewayReader interface {
	ListGateways(ctx context.Context, query string, limit int32) (*adminv1.ListGatewaysResponse, error)
}

// RouteReader 是路由列表工具所需的最小查询边界。
type RouteReader interface {
	ListRoutes(ctx context.Context, query string, limit int32) (*adminv1.ListRoutesResponse, error)
}

// ServiceReader 是服务列表工具所需的最小查询边界。
type ServiceReader interface {
	ListServices(ctx context.Context, query string, limit int32) (*adminv1.ListUpstreamsResponse, error)
}

// TrafficReader 是聚合流量工具所需的查询边界。
type TrafficReader interface {
	GetTrafficAnalysis(ctx context.Context, request *adminv1.GetTrafficAnalysisRequest) (*adminv1.GetTrafficAnalysisResponse, error)
}

// FailureReader 是失败请求工具所需的查询边界。
type FailureReader interface {
	ListRequestRecords(ctx context.Context, request *adminv1.ListRequestRecordsRequest) (*adminv1.ListRequestRecordsResponse, error)
}

// OperationsSource 明确列出运维 Agent 当前需要的所有外部查询能力。
// 单个工具只接收自己的窄接口；这里仅作为进程装配点组合这些能力。
type OperationsSource interface {
	GatewayReader
	RouteReader
	ServiceReader
	TrafficReader
	FailureReader
}

type listResourcesInput struct {
	Query string `json:"query,omitempty"`
	Limit int32  `json:"limit,omitempty"`
}

// NewOperations 创建运维 Agent 当前可以提供给模型的只读工具。
// 返回顺序保持稳定，便于观察不同版本暴露给模型的能力变化。
func NewOperations(source OperationsSource) ([]einotool.BaseTool, error) {
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
	traffic, err := newTrafficTool(source)
	if err != nil {
		return nil, err
	}
	failures, err := newFailureTool(source)
	if err != nil {
		return nil, err
	}
	return []einotool.BaseTool{gateways, routes, services, traffic, failures}, nil
}

func listLimit(limit int32) int32 {
	if limit <= 0 {
		return defaultListLimit
	}
	return min(limit, maxListLimit)
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

func resultStatus(hasMore bool) string {
	if hasMore {
		return "partial"
	}
	return "complete"
}
