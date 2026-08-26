package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
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
	result, err := resources.ListRoutes(ctx, strings.TrimSpace(input.Query), listLimit(input.Limit))
	if err != nil {
		return routeToolOutput{}, err
	}
	items := make([]routeInfo, 0, len(result.GetRoutes()))
	for _, route := range result.GetRoutes() {
		serviceIDs := make([]string, 0, len(route.GetUpstreams()))
		for _, target := range route.GetUpstreams() {
			serviceIDs = append(serviceIDs, target.GetUpstreamId())
		}
		var models []string
		if route.GetAi() != nil {
			for _, model := range route.GetAi().GetModels() {
				models = append(models, model.GetName())
				for _, target := range model.GetTargets() {
					serviceIDs = appendUnique(serviceIDs, target.GetUpstreamId())
				}
			}
		}
		path := ""
		if route.GetMatch() != nil && route.GetMatch().GetPath() != nil {
			path = route.GetMatch().GetPath().GetValue()
		}
		items = append(items, routeInfo{
			ID:            route.GetId(),
			Name:          route.GetName(),
			Type:          routeType(route),
			Enabled:       route.GetEnabled(),
			State:         resourceState(route.GetState()),
			Message:       route.GetMessage(),
			AccessMode:    routeAccessMode(route.GetAccessMode()),
			GatewayIDs:    route.GetGatewayIds(),
			Path:          path,
			ServiceIDs:    serviceIDs,
			ExposedModels: models,
		})
	}
	hasMore := result.GetNextCursor() != ""
	return routeToolOutput{
		Summary: fmt.Sprintf("找到 %d 条路由", len(items)),
		Source:  "admin_api",
		Status:  resultStatus(hasMore),
		HasMore: hasMore,
		Items:   items,
	}, nil
}

func routeAccessMode(mode adminv1.RouteAccessMode) string {
	if mode == adminv1.RouteAccessMode_ROUTE_ACCESS_CALLER {
		return "caller"
	}
	return "public"
}

func routeType(route *adminv1.Route) string {
	if route.GetAi() != nil {
		return "ai"
	}
	return "api"
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}
