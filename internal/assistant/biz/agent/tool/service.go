package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type serviceToolOutput struct {
	Summary string        `json:"summary"`
	Source  string        `json:"source"`
	Status  string        `json:"status"`
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

func newServiceTool(resources ServiceReader) (einotool.BaseTool, error) {
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
	resources ServiceReader,
	input listResourcesInput,
) (serviceToolOutput, error) {
	result, err := resources.ListServices(ctx, ResourceListQuery{
		Text:  strings.TrimSpace(input.Query),
		Limit: listLimit(input.Limit),
	})
	if err != nil {
		return serviceToolOutput{}, err
	}
	items := make([]serviceInfo, 0, len(result.Items))
	for _, service := range result.Items {
		items = append(items, serviceInfoFromResource(service))
	}
	return serviceToolOutput{
		Summary: fmt.Sprintf("找到 %d 个服务", len(items)),
		Source:  "admin_api",
		Status:  resultStatus(result.HasMore),
		HasMore: result.HasMore,
		Items:   items,
	}, nil
}

func serviceInfoFromResource(service Service) serviceInfo {
	return serviceInfo(service)
}
