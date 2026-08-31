package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type routeToolOutput struct {
	Summary string      `json:"summary"`
	Source  string      `json:"source"`
	Status  string      `json:"status"`
	HasMore bool        `json:"has_more"`
	Items   []routeInfo `json:"items"`
}

type routeInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Enabled       bool     `json:"enabled"`
	State         string   `json:"state"`
	Message       string   `json:"message,omitempty"`
	AccessMode    string   `json:"access_mode"`
	GatewayIDs    []string `json:"gateway_ids"`
	Path          string   `json:"path"`
	ServiceIDs    []string `json:"service_ids"`
	ExposedModels []string `json:"exposed_models,omitempty"`
}

// RouteReader 是路由列表工具实际使用的查询边界。
type RouteReader interface {
	ListRoutes(context.Context, ResourceListQuery) (RoutePage, error)
}

func newRouteTool(resources RouteReader) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		listRoutesTool,
		"查询当前 Ingate 环境中的 API 路由和 AI 路由及其网关、服务关联。",
		func(ctx context.Context, input listResourcesInput) (routeToolOutput, error) {
			return listRoutes(ctx, resources, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", listRoutesTool, err)
	}
	return definition, nil
}

func listRoutes(
	ctx context.Context,
	resources RouteReader,
	input listResourcesInput,
) (routeToolOutput, error) {
	result, err := resources.ListRoutes(ctx, ResourceListQuery{
		Text:  strings.TrimSpace(input.Query),
		Limit: listLimit(input.Limit),
	})
	if err != nil {
		return routeToolOutput{}, err
	}
	items := make([]routeInfo, 0, len(result.Items))
	for _, route := range result.Items {
		items = append(items, routeInfo(route))
	}
	return routeToolOutput{
		Summary: fmt.Sprintf("找到 %d 条路由", len(items)),
		Source:  "admin_api",
		Status:  resultStatus(result.HasMore),
		HasMore: result.HasMore,
		Items:   items,
	}, nil
}
