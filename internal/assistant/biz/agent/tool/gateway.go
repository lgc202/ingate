package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type gatewayToolOutput struct {
	Summary string        `json:"summary"`
	Source  string        `json:"source"`
	Status  string        `json:"status"`
	HasMore bool          `json:"has_more"`
	Items   []gatewayInfo `json:"items"`
}

type gatewayInfo struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Enabled   bool           `json:"enabled"`
	State     string         `json:"state"`
	Message   string         `json:"message,omitempty"`
	Listeners []listenerInfo `json:"listeners"`
}

// GatewayReader 是网关列表工具实际使用的查询边界。
type GatewayReader interface {
	ListGateways(context.Context, ResourceListQuery) (GatewayPage, error)
}

type listenerInfo struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Port     uint32 `json:"port"`
	Hostname string `json:"hostname,omitempty"`
}

func newGatewayTool(resources GatewayReader) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		listGatewaysTool,
		"查询当前 Ingate 环境中的网关入口。可按名称、域名或监听入口搜索。",
		func(ctx context.Context, input listResourcesInput) (gatewayToolOutput, error) {
			return listGateways(ctx, resources, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", listGatewaysTool, err)
	}
	return definition, nil
}

func listGateways(
	ctx context.Context,
	resources GatewayReader,
	input listResourcesInput,
) (gatewayToolOutput, error) {
	result, err := resources.ListGateways(ctx, ResourceListQuery{
		Text:  strings.TrimSpace(input.Query),
		Limit: listLimit(input.Limit),
	})
	if err != nil {
		return gatewayToolOutput{}, err
	}
	items := make([]gatewayInfo, 0, len(result.Items))
	for _, gateway := range result.Items {
		items = append(items, gatewayInfoFromResource(gateway))
	}
	return gatewayToolOutput{
		Summary: fmt.Sprintf("找到 %d 个网关", len(items)),
		Source:  "admin_api",
		Status:  resultStatus(result.HasMore),
		HasMore: result.HasMore,
		Items:   items,
	}, nil
}

func gatewayInfoFromResource(gateway Gateway) gatewayInfo {
	listeners := make([]listenerInfo, 0, len(gateway.Listeners))
	for _, listener := range gateway.Listeners {
		listeners = append(listeners, listenerInfo(listener))
	}
	return gatewayInfo{
		ID:        gateway.ID,
		Name:      gateway.Name,
		Enabled:   gateway.Enabled,
		State:     gateway.State,
		Message:   gateway.Message,
		Listeners: listeners,
	}
}
