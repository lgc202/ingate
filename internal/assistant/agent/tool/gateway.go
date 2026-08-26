package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
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
	result, err := resources.ListGateways(ctx, strings.TrimSpace(input.Query), listLimit(input.Limit))
	if err != nil {
		return gatewayToolOutput{}, err
	}
	items := make([]gatewayInfo, 0, len(result.GetGateways()))
	for _, gateway := range result.GetGateways() {
		listeners := make([]listenerInfo, 0, len(gateway.GetListeners()))
		for _, listener := range gateway.GetListeners() {
			listeners = append(listeners, listenerInfo{
				Name:     listener.GetName(),
				Protocol: gatewayProtocol(listener.GetProtocol()),
				Port:     listener.GetPort(),
				Hostname: listener.GetHostname(),
			})
		}
		items = append(items, gatewayInfo{
			ID:        gateway.GetId(),
			Name:      gateway.GetName(),
			Enabled:   gateway.GetEnabled(),
			State:     resourceState(gateway.GetState()),
			Message:   gateway.GetMessage(),
			Listeners: listeners,
		})
	}
	hasMore := result.GetNextCursor() != ""
	return gatewayToolOutput{
		Summary: fmt.Sprintf("找到 %d 个网关", len(items)),
		Source:  "admin_api",
		Status:  resultStatus(hasMore),
		HasMore: hasMore,
		Items:   items,
	}, nil
}

func gatewayProtocol(protocol adminv1.GatewayProtocol) string {
	if protocol == adminv1.GatewayProtocol_GATEWAY_PROTOCOL_HTTPS {
		return "https"
	}
	return "http"
}
