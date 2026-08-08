package service

import (
	"strconv"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func upstreamReply(upstream *resource.Upstream) *adminv1.Upstream {
	reply := &adminv1.Upstream{
		Id:                upstream.Name,
		Version:           strconv.FormatInt(upstream.Generation, 10),
		Status:            resourceStatus(biz.ResourceStatusFromConditions(upstream.Generation, upstream.Status.Conditions)),
		ApiKeyConfigured:  upstream.Spec.Authentication != nil && upstream.Spec.Authentication.APIKey != nil && upstream.Spec.Authentication.APIKey.Value != "",
		Name:              upstream.Spec.DisplayName,
		Type:              string(upstream.Spec.Type),
		Protocol:          string(upstream.Spec.Protocol),
		LoadBalancePolicy: string(upstream.Spec.LoadBalancePolicy),
		CreatedAt:         timestamp(upstream.CreationTimestamp.Time),
	}
	if reply.LoadBalancePolicy == "" {
		reply.LoadBalancePolicy = string(resource.UpstreamLoadBalancePolicyRoundRobin)
	}
	if upstream.Spec.TLS != nil {
		reply.Tls = &adminv1.UpstreamTLS{ServerName: upstream.Spec.TLS.ServerName}
	}
	if upstream.Spec.Model != nil {
		reply.Model = &adminv1.ModelConfig{Provider: string(upstream.Spec.Model.Provider), ApiBasePath: upstream.Spec.Model.APIBasePath}
		for _, model := range upstream.Spec.Model.Models {
			reply.Model.Models = append(reply.Model.Models, &adminv1.ModelCatalogItem{
				Name: model.Name, DisplayName: model.DisplayName, Enabled: model.Enabled,
			})
		}
	}
	for _, endpoint := range upstream.Spec.Endpoints {
		reply.Endpoints = append(reply.Endpoints, &adminv1.UpstreamEndpoint{
			Id: endpoint.Name, Address: endpoint.Address, Port: int32(endpoint.Port),
			Weight: int32(endpoint.Weight), Enabled: endpoint.Enabled,
		})
	}
	if upstream.Spec.HealthCheck != nil {
		reply.HealthCheck = &adminv1.UpstreamHealthCheck{
			Enabled: upstream.Spec.HealthCheck.Enabled, Path: upstream.Spec.HealthCheck.Path,
			IntervalSeconds: int32(upstream.Spec.HealthCheck.IntervalSeconds),
			TimeoutSeconds:  int32(upstream.Spec.HealthCheck.TimeoutSeconds),
		}
	}
	return reply
}
