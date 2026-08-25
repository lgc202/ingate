package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

type serviceToolOutput struct {
	Summary string        `json:"summary"`
	HasMore bool          `json:"has_more"`
	Items   []serviceInfo `json:"items"`
}

type serviceInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	State         string `json:"state"`
	Message       string `json:"message,omitempty"`
	EndpointCount int    `json:"endpoint_count"`
	TLS           bool   `json:"tls"`
	ModelProtocol string `json:"model_protocol,omitempty"`
}

func newServiceTool(resources ResourceReader) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		listServicesTool,
		"查询当前 Ingate 环境中的普通服务和模型服务。不会返回服务凭据或具体地址。",
		func(ctx context.Context, input listResourcesInput) (serviceToolOutput, error) {
			return listServices(ctx, resources, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", listServicesTool, err)
	}
	return definition, nil
}

func listServices(
	ctx context.Context,
	resources ResourceReader,
	input listResourcesInput,
) (serviceToolOutput, error) {
	result, err := resources.ListServices(ctx, strings.TrimSpace(input.Query), listLimit(input.Limit))
	if err != nil {
		return serviceToolOutput{}, err
	}
	items := make([]serviceInfo, 0, len(result.GetUpstreams()))
	for _, service := range result.GetUpstreams() {
		info := serviceInfo{
			ID:            service.GetId(),
			Name:          service.GetName(),
			Type:          "http",
			State:         resourceState(service.GetState()),
			Message:       service.GetMessage(),
			EndpointCount: len(service.GetEndpoints()),
			TLS:           service.GetTls() != nil,
		}
		if service.GetModel() != nil {
			info.Type = "model"
			info.ModelProtocol = modelProtocol(service.GetModel().GetProtocol())
		}
		items = append(items, info)
	}
	return serviceToolOutput{
		Summary: fmt.Sprintf("找到 %d 个服务", len(items)),
		HasMore: result.GetNextCursor() != "",
		Items:   items,
	}, nil
}

func modelProtocol(protocol adminv1.ModelProtocol) string {
	if protocol == adminv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC {
		return "anthropic"
	}
	return "openai"
}
