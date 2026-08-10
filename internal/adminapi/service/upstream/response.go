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
		Id:               upstream.Name,
		Name:             upstream.Spec.DisplayName,
		Type:             upstreamTypeFromResource(upstream.Spec.Type),
		LoadBalancing:    loadBalancingFromResource(upstream.Spec.LoadBalancing),
		ApiKeyConfigured: upstream.Spec.Model != nil && upstream.Spec.Model.APIKey != "",
		State:            adminservice.NewResourceState(status.State),
		Message:          adminservice.ResourceMessage(status.Reason),
		Version:          upstream.Generation,
		CreatedAt:        adminservice.NewTimestamp(upstream.CreationTimestamp.Time),
		UpdatedAt:        adminservice.NewTimestamp(upstreamUpdatedAt(upstream)),
		Endpoints:        make([]*adminv1.UpstreamEndpoint, 0, len(upstream.Spec.Endpoints)),
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
	if upstream.Spec.Model != nil {
		response.Model = &adminv1.ModelConfig{
			Provider: modelProviderFromResource(upstream.Spec.Model.Provider),
			BasePath: upstream.Spec.Model.BasePath,
			Models:   append([]string(nil), upstream.Spec.Model.Models...),
		}
	}
	return response
}

func upstreamTypeFromResource(upstreamType resource.UpstreamType) adminv1.UpstreamType {
	switch upstreamType {
	case resource.UpstreamTypeApplication:
		return adminv1.UpstreamType_UPSTREAM_TYPE_APPLICATION
	case resource.UpstreamTypeModel:
		return adminv1.UpstreamType_UPSTREAM_TYPE_MODEL
	case resource.UpstreamTypeAgent:
		return adminv1.UpstreamType_UPSTREAM_TYPE_AGENT
	case resource.UpstreamTypeMCP:
		return adminv1.UpstreamType_UPSTREAM_TYPE_MCP
	default:
		return adminv1.UpstreamType_UPSTREAM_TYPE_UNSPECIFIED
	}
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

func modelProviderFromResource(provider resource.ModelProvider) adminv1.ModelProvider {
	switch provider {
	case resource.ModelProviderOpenAI:
		return adminv1.ModelProvider_MODEL_PROVIDER_OPENAI
	case resource.ModelProviderDeepSeek:
		return adminv1.ModelProvider_MODEL_PROVIDER_DEEPSEEK
	case resource.ModelProviderQwen:
		return adminv1.ModelProvider_MODEL_PROVIDER_QWEN
	case resource.ModelProviderAnthropic:
		return adminv1.ModelProvider_MODEL_PROVIDER_ANTHROPIC
	case resource.ModelProviderGemini:
		return adminv1.ModelProvider_MODEL_PROVIDER_GEMINI
	case resource.ModelProviderCustom:
		return adminv1.ModelProvider_MODEL_PROVIDER_CUSTOM
	default:
		return adminv1.ModelProvider_MODEL_PROVIDER_UNSPECIFIED
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
