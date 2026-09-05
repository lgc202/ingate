package servicemanagement

import (
	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func serviceResponse(service *resource.Upstream) *adminv1.Service {
	status := resourceview.StatusFromConditions(
		service.Generation,
		service.Status.Conditions,
	)
	endpoints := lo.Map(service.Spec.Endpoints, func(endpoint resource.Endpoint, _ int) *adminv1.ServiceEndpoint {
		return &adminv1.ServiceEndpoint{
			Address: endpoint.Address,
			Port:    uint32(endpoint.Port),
			Weight:  uint32(endpoint.Weight),
		}
	})
	response := &adminv1.Service{
		Id:            service.Name,
		Name:          service.Spec.DisplayName,
		Endpoints:     endpoints,
		LoadBalancing: loadBalancingResponse(service.Spec.LoadBalancing),
		State:         adminservice.ResourceState(status.State),
		Message:       adminservice.ResourceMessage(status.Reason),
		Version:       service.Generation,
		CreatedAt:     adminservice.Timestamp(service.CreationTimestamp.Time),
		UpdatedAt: adminservice.Timestamp(
			adminservice.ResourceUpdatedAt(service.Annotations),
		),
	}
	if service.Spec.TLS != nil {
		response.Tls = &adminv1.ServiceTLS{ServerName: service.Spec.TLS.ServerName}
	}
	if service.Spec.HealthCheck != nil {
		response.HealthCheck = &adminv1.ServiceHealthCheck{
			Path:            service.Spec.HealthCheck.Path,
			IntervalSeconds: uint32(service.Spec.HealthCheck.IntervalSeconds),
			TimeoutSeconds:  uint32(service.Spec.HealthCheck.TimeoutSeconds),
		}
	}
	if service.Spec.Model != nil {
		response.Model = &adminv1.ModelService{
			Protocol:         modelProtocolResponse(service.Spec.Model.Protocol),
			ApiKeyConfigured: service.Spec.Model.APIKey != "",
		}
	}
	return response
}

func modelProtocolResponse(protocol resource.ModelProtocol) adminv1.ModelProtocol {
	switch protocol {
	case resource.ModelProtocolOpenAI:
		return adminv1.ModelProtocol_MODEL_PROTOCOL_OPENAI
	case resource.ModelProtocolAnthropic:
		return adminv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC
	default:
		return adminv1.ModelProtocol_MODEL_PROTOCOL_UNSPECIFIED
	}
}

func loadBalancingResponse(policy resource.LoadBalancingPolicy) adminv1.LoadBalancingPolicy {
	switch policy {
	case resource.LoadBalancingRoundRobin:
		return adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_ROUND_ROBIN
	case resource.LoadBalancingLeastRequest:
		return adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_LEAST_REQUEST
	default:
		return adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_UNSPECIFIED
	}
}
