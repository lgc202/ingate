package tool

import einotool "github.com/cloudwego/eino/components/tool"

const (
	listGatewaysTool       = "list_gateways"
	listRoutesTool         = "list_routes"
	listServicesTool       = "list_services"
	getRecentTrafficTool   = "get_recent_traffic"
	listRecentFailuresTool = "list_recent_failures"
	defaultListLimit       = 20
	maxListLimit           = 50
)

type listResourcesInput struct {
	Query string `json:"query,omitempty"`
	Limit int32  `json:"limit,omitempty"`
}

// NewTools 创建运维 Agent 当前可以提供给模型的只读工具。
// 返回顺序保持稳定，便于观察不同版本暴露给模型的能力变化。
func NewTools(source QuerySource) ([]einotool.BaseTool, error) {
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

func resultStatus(hasMore bool) string {
	if hasMore {
		return "partial"
	}
	return "complete"
}
