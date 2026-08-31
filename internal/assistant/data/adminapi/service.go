package adminapi

import (
	"context"
	"errors"
	"fmt"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
)

// CreateService 执行管理员已经批准的普通 HTTP Service 创建操作。
func (c *Client) CreateService(
	ctx context.Context,
	input changebiz.CreateService,
) (changebiz.CreatedResource, error) {
	endpoints := make([]*adminv1.ServiceEndpoint, 0, len(input.Endpoints))
	for _, endpoint := range input.Endpoints {
		endpoints = append(endpoints, &adminv1.ServiceEndpoint{
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  endpoint.Weight,
		})
	}
	request := &adminv1.CreateServiceRequest{
		Name:          input.Name,
		Endpoints:     endpoints,
		LoadBalancing: proposedLoadBalancing(input.LoadBalancing),
	}
	if input.TLSServerName != "" {
		request.Tls = &adminv1.ServiceTLS{ServerName: input.TLSServerName}
	}
	if input.HealthCheck != nil {
		request.HealthCheck = &adminv1.ServiceHealthCheck{
			Path:            input.HealthCheck.Path,
			IntervalSeconds: input.HealthCheck.IntervalSeconds,
			TimeoutSeconds:  input.HealthCheck.TimeoutSeconds,
		}
	}
	result, err := c.services.CreateService(ctx, request)
	if err != nil {
		return changebiz.CreatedResource{}, proposedChangeError("create service through Admin API", err)
	}
	if result == nil || !validResourceID(result.GetId()) {
		return changebiz.CreatedResource{}, errors.New("create service through Admin API: invalid response")
	}
	return changebiz.CreatedResource{ID: result.GetId()}, nil
}

// ListServices 查询当前配置域中的普通服务和模型服务。
func (c *Client) ListServices(
	ctx context.Context,
	query agenttool.ResourceListQuery,
) (agenttool.ServicePage, error) {
	result, err := c.services.ListServices(ctx, &adminv1.ListServicesRequest{
		Query: query.Text,
		Limit: query.Limit,
	})
	if err != nil {
		return agenttool.ServicePage{}, fmt.Errorf("list services from Admin API: %w", err)
	}
	if result == nil {
		return agenttool.ServicePage{}, errors.New("list services from Admin API: empty response")
	}

	services := make([]agenttool.Service, 0, len(result.GetServices()))
	for _, service := range result.GetServices() {
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

func serviceFromAPI(service *adminv1.Service) agenttool.Service {
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

func serviceType(service *adminv1.Service) string {
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

func proposedLoadBalancing(policy changebiz.LoadBalancing) adminv1.LoadBalancingPolicy {
	switch policy {
	case changebiz.LoadBalancingRoundRobin:
		return adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_ROUND_ROBIN
	case changebiz.LoadBalancingLeastRequest:
		return adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_LEAST_REQUEST
	default:
		return adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_UNSPECIFIED
	}
}

func validateServiceResponse(service *adminv1.Service) error {
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
