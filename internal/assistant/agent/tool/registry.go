// Package tool 定义模型 Agent 可调用的 Ingate 原子工具。
package tool

import (
	"context"
	"fmt"
	"slices"

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

// ResourceReader 是只读工具访问 Ingate 资源所需的最小边界。
type ResourceReader interface {
	ListGateways(ctx context.Context, query string, limit int32) (*adminv1.ListGatewaysResponse, error)
	ListRoutes(ctx context.Context, query string, limit int32) (*adminv1.ListRoutesResponse, error)
	ListServices(ctx context.Context, query string, limit int32) (*adminv1.ListUpstreamsResponse, error)
	GetTrafficAnalysis(ctx context.Context, request *adminv1.GetTrafficAnalysisRequest) (*adminv1.GetTrafficAnalysisResponse, error)
	ListRequestRecords(ctx context.Context, request *adminv1.ListRequestRecordsRequest) (*adminv1.ListRequestRecordsResponse, error)
}

// Registry 保存 Assistant 可提供给模型的工具定义。
type Registry struct {
	tools []einotool.BaseTool
	names map[string]struct{}
}

type listResourcesInput struct {
	Query string `json:"query,omitempty"`
	Limit int32  `json:"limit,omitempty"`
}

// NewRegistry 创建当前版本支持的只读工具集合。
func NewRegistry(resources ResourceReader) (*Registry, error) {
	gateways, err := newGatewayTool(resources)
	if err != nil {
		return nil, err
	}
	routes, err := newRouteTool(resources)
	if err != nil {
		return nil, err
	}
	services, err := newServiceTool(resources)
	if err != nil {
		return nil, err
	}
	traffic, err := newTrafficTool(resources)
	if err != nil {
		return nil, err
	}
	failures, err := newFailureTool(resources)
	if err != nil {
		return nil, err
	}
	return &Registry{
		tools: []einotool.BaseTool{gateways, routes, services, traffic, failures},
		names: map[string]struct{}{
			listGatewaysTool:       {},
			listRoutesTool:         {},
			listServicesTool:       {},
			getRecentTrafficTool:   {},
			listRecentFailuresTool: {},
		},
	}, nil
}

// All 返回按稳定顺序排列的全部工具。
func (r *Registry) All() []einotool.BaseTool {
	return slices.Clone(r.tools)
}

// Validate 检查 Skill 声明的工具是否已经注册。
func (r *Registry) Validate(names []string) error {
	for _, name := range names {
		if _, ok := r.names[name]; !ok {
			return fmt.Errorf("assistant tool %q is not registered", name)
		}
	}
	return nil
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
