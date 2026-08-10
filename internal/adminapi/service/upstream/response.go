package upstream

import (
	"time"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func upstreamFromResource(upstream *resource.Upstream) *adminv1.Upstream {
	status := biz.ResourceStatusFromConditions(upstream.Generation, upstream.Status.Conditions)
	response := &adminv1.Upstream{
		Id:            upstream.Name,
		Name:          upstream.Spec.DisplayName,
		LoadBalancing: loadBalancingFromResource(upstream.Spec.LoadBalancing),
		State:         adminservice.NewResourceState(status.State),
		Message:       adminservice.ResourceMessage(status.Reason),
		Version:       upstream.Generation,
		CreatedAt:     adminservice.NewTimestamp(upstream.CreationTimestamp.Time),
		UpdatedAt:     adminservice.NewTimestamp(upstreamUpdatedAt(upstream)),
		Endpoints:     make([]*adminv1.UpstreamEndpoint, 0, len(upstream.Spec.Endpoints)),
	}
	for _, endpoint := range upstream.Spec.Endpoints {
		response.Endpoints = append(response.Endpoints, &adminv1.UpstreamEndpoint{
			Address: endpoint.Address,
			Port:    uint32(endpoint.Port),
			Weight:  uint32(endpoint.Weight),
		})
	}
	if upstream.Spec.TLS != nil {
		response.Tls = &adminv1.UpstreamTLS{ServerName: upstream.Spec.TLS.ServerName}
	}
	if upstream.Spec.HealthCheck != nil {
		response.HealthCheck = &adminv1.UpstreamHealthCheck{
			Path:            upstream.Spec.HealthCheck.Path,
			IntervalSeconds: uint32(upstream.Spec.HealthCheck.IntervalSeconds),
			TimeoutSeconds:  uint32(upstream.Spec.HealthCheck.TimeoutSeconds),
		}
	}
	return response
}

func loadBalancingFromResource(policy resource.LoadBalancingPolicy) adminv1.LoadBalancingPolicy {
	switch policy {
	case resource.LoadBalancingRoundRobin:
		return adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_ROUND_ROBIN
	case resource.LoadBalancingLeastRequest:
		return adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_LEAST_REQUEST
	default:
		return adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_UNSPECIFIED
	}
}

func upstreamUpdatedAt(upstream *resource.Upstream) time.Time {
	value := upstream.Annotations[resource.AnnotationUpdatedAt]
	if value == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
