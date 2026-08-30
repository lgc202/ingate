package adminapi

import (
	"context"
	"errors"
	"fmt"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
)

// ListServices 查询当前配置域中的普通服务和模型服务。
func (c *Client) ListServices(
	ctx context.Context,
	query agenttool.ResourceListQuery,
) (agenttool.ServicePage, error) {
	result, err := c.services.ListUpstreams(ctx, &adminv1.ListUpstreamsRequest{
		Query: query.Text,
		Limit: query.Limit,
	})
	if err != nil {
		return agenttool.ServicePage{}, fmt.Errorf("list services from Admin API: %w", err)
	}
	if result == nil {
		return agenttool.ServicePage{}, errors.New("list services from Admin API: empty response")
	}

	services := make([]agenttool.Service, 0, len(result.GetUpstreams()))
	for _, service := range result.GetUpstreams() {
		if err := validateServiceResponse(service); err != nil {
			return agenttool.ServicePage{}, err
		}
		services = append(services, serviceFromAPI(service))
	}
	return agenttool.ServicePage{
		Items:   services,
		HasMore: result.GetNextCursor() != "",
	}, nil
}

func serviceFromAPI(service *adminv1.Upstream) agenttool.Service {
	return agenttool.Service{
		ID:            service.GetId(),
		Name:          service.GetName(),
		Type:          serviceType(service),
		State:         resourceState(service.GetState()),
		Message:       service.GetMessage(),
		EndpointCount: len(service.GetEndpoints()),
		TLS:           service.GetTls() != nil,
		ModelProtocol: modelProtocol(service.GetModel().GetProtocol()),
	}
}

func serviceType(service *adminv1.Upstream) string {
	if service.GetModel() != nil {
		return "model"
	}
	return "http"
}

func modelProtocol(protocol adminv1.ModelProtocol) string {
	switch protocol {
	case adminv1.ModelProtocol_MODEL_PROTOCOL_OPENAI:
		return "openai"
	case adminv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC:
		return "anthropic"
	default:
		return ""
	}
}

func validateServiceResponse(service *adminv1.Upstream) error {
	if service == nil || !validResourceID(service.GetId()) || service.GetName() == "" ||
		len(service.GetEndpoints()) == 0 {
		return errors.New("invalid service returned by Admin API")
	}
	if !validResourceState(service.GetState()) {
		return fmt.Errorf("service %s returned an invalid resource state", service.GetId())
	}
	if model := service.GetModel(); model != nil &&
		model.GetProtocol() != adminv1.ModelProtocol_MODEL_PROTOCOL_OPENAI &&
		model.GetProtocol() != adminv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC {
		return fmt.Errorf("service %s returned an invalid model protocol", service.GetId())
	}
	for _, endpoint := range service.GetEndpoints() {
		if endpoint == nil || endpoint.GetAddress() == "" || endpoint.GetPort() == 0 ||
			endpoint.GetWeight() == 0 {
			return fmt.Errorf("service %s returned an invalid endpoint", service.GetId())
		}
	}
	return nil
}
