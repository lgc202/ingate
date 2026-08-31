package change

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
)

func changeResponse(item changebiz.ProposedChange) *assistantv1.ProposedChange {
	response := &assistantv1.ProposedChange{
		Id:             item.ID,
		ConversationId: item.ConversationID,
		ExecutionId:    item.ExecutionID,
		Kind:           changeKind(item.Proposal.Kind),
		State:          changeState(item.State),
		Summary:        item.Summary,
		ResourceId:     item.ResourceID,
		ErrorCode:      string(item.ErrorCode),
		CreatedAt:      timestamppb.New(item.CreatedAt),
	}
	if item.Proposal.Gateway != nil {
		response.Configuration = &assistantv1.ProposedChange_CreateGateway{
			CreateGateway: gatewayResponse(*item.Proposal.Gateway),
		}
	}
	if item.Proposal.Service != nil {
		response.Configuration = &assistantv1.ProposedChange_CreateService{
			CreateService: serviceResponse(*item.Proposal.Service),
		}
	}
	if item.DecidedAt != nil {
		response.DecidedAt = timestamppb.New(*item.DecidedAt)
	}
	if item.FinishedAt != nil {
		response.FinishedAt = timestamppb.New(*item.FinishedAt)
	}
	return response
}

func gatewayResponse(input changebiz.CreateGateway) *assistantv1.ProposedGateway {
	listeners := make([]*assistantv1.ProposedGatewayListener, 0, len(input.Listeners))
	for _, listener := range input.Listeners {
		listeners = append(listeners, &assistantv1.ProposedGatewayListener{
			Name:          listener.Name,
			Protocol:      gatewayProtocol(listener.Protocol),
			Port:          listener.Port,
			Hostname:      listener.Hostname,
			CertificateId: listener.CertificateID,
		})
	}
	return &assistantv1.ProposedGateway{
		Name:      input.Name,
		Enabled:   input.Enabled,
		Listeners: listeners,
	}
}

func serviceResponse(input changebiz.CreateService) *assistantv1.ProposedService {
	endpoints := make([]*assistantv1.ProposedServiceEndpoint, 0, len(input.Endpoints))
	for _, endpoint := range input.Endpoints {
		endpoints = append(endpoints, &assistantv1.ProposedServiceEndpoint{
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  endpoint.Weight,
		})
	}
	response := &assistantv1.ProposedService{
		Name:          input.Name,
		Endpoints:     endpoints,
		TlsServerName: input.TLSServerName,
		LoadBalancing: serviceLoadBalancing(input.LoadBalancing),
	}
	if input.HealthCheck != nil {
		response.HealthCheck = &assistantv1.ProposedServiceHealthCheck{
			Path:            input.HealthCheck.Path,
			IntervalSeconds: input.HealthCheck.IntervalSeconds,
			TimeoutSeconds:  input.HealthCheck.TimeoutSeconds,
		}
	}
	return response
}

func changeKind(kind changebiz.Kind) assistantv1.ProposedChangeKind {
	switch kind {
	case changebiz.KindCreateGateway:
		return assistantv1.ProposedChangeKind_PROPOSED_CHANGE_KIND_CREATE_GATEWAY
	case changebiz.KindCreateService:
		return assistantv1.ProposedChangeKind_PROPOSED_CHANGE_KIND_CREATE_SERVICE
	default:
		return assistantv1.ProposedChangeKind_PROPOSED_CHANGE_KIND_UNSPECIFIED
	}
}

func changeState(state changebiz.State) assistantv1.ProposedChangeState {
	switch state {
	case changebiz.StatePendingReview:
		return assistantv1.ProposedChangeState_PROPOSED_CHANGE_STATE_PENDING_REVIEW
	case changebiz.StateExecuting:
		return assistantv1.ProposedChangeState_PROPOSED_CHANGE_STATE_EXECUTING
	case changebiz.StateSucceeded:
		return assistantv1.ProposedChangeState_PROPOSED_CHANGE_STATE_SUCCEEDED
	case changebiz.StateRejected:
		return assistantv1.ProposedChangeState_PROPOSED_CHANGE_STATE_REJECTED
	case changebiz.StateFailed:
		return assistantv1.ProposedChangeState_PROPOSED_CHANGE_STATE_FAILED
	case changebiz.StateOutcomeUnknown:
		return assistantv1.ProposedChangeState_PROPOSED_CHANGE_STATE_OUTCOME_UNKNOWN
	default:
		return assistantv1.ProposedChangeState_PROPOSED_CHANGE_STATE_UNSPECIFIED
	}
}

func gatewayProtocol(protocol changebiz.GatewayProtocol) assistantv1.ProposedGatewayProtocol {
	switch protocol {
	case changebiz.GatewayProtocolHTTP:
		return assistantv1.ProposedGatewayProtocol_PROPOSED_GATEWAY_PROTOCOL_HTTP
	case changebiz.GatewayProtocolHTTPS:
		return assistantv1.ProposedGatewayProtocol_PROPOSED_GATEWAY_PROTOCOL_HTTPS
	default:
		return assistantv1.ProposedGatewayProtocol_PROPOSED_GATEWAY_PROTOCOL_UNSPECIFIED
	}
}

func serviceLoadBalancing(
	policy changebiz.LoadBalancing,
) assistantv1.ProposedServiceLoadBalancing {
	switch policy {
	case changebiz.LoadBalancingRoundRobin:
		return assistantv1.ProposedServiceLoadBalancing_PROPOSED_SERVICE_LOAD_BALANCING_ROUND_ROBIN
	case changebiz.LoadBalancingLeastRequest:
		return assistantv1.ProposedServiceLoadBalancing_PROPOSED_SERVICE_LOAD_BALANCING_LEAST_REQUEST
	default:
		return assistantv1.ProposedServiceLoadBalancing_PROPOSED_SERVICE_LOAD_BALANCING_UNSPECIFIED
	}
}
