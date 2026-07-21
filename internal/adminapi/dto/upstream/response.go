package upstream

import (
	"strconv"
	"time"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	"github.com/lgc202/ingate/internal/adminapi/service/resourcestatus"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewListUpstreamsResp 转换 Upstream 资源列表为控制台服务列表响应
func NewListUpstreamsResp(resources []resource.Upstream) ListUpstreamsResp {
	return ListUpstreamsResp{
		Upstreams: lo.Map(resources, func(upstream resource.Upstream, _ int) Upstream {
			return upstreamFromResource(&upstream)
		}),
	}
}

// NewGetUpstreamResp 转换 Upstream 资源为控制台服务响应
func NewGetUpstreamResp(upstream *resource.Upstream) Upstream {
	return upstreamFromResource(upstream)
}

func upstreamFromResource(upstream *resource.Upstream) Upstream {
	return Upstream{
		ID:               upstream.Name,
		Version:          strconv.FormatInt(upstream.Generation, 10),
		Status:           admindto.NewResourceStatus(resourcestatus.FromConditions(upstream.Generation, upstream.Status.Conditions)),
		APIKeyConfigured: upstream.Spec.Authentication != nil && upstream.Spec.Authentication.APIKey != nil && upstream.Spec.Authentication.APIKey.Value != "",
		UpstreamConfig: UpstreamConfig{
			Name:              upstreamName(upstream),
			Type:              upstream.Spec.Type,
			Protocol:          upstream.Spec.Protocol,
			TLS:               upstreamTLS(upstream.Spec.TLS),
			Model:             modelConfig(upstream.Spec.Model),
			Endpoints:         endpointRequests(upstream),
			LoadBalancePolicy: loadBalancePolicy(upstream.Spec.LoadBalancePolicy),
			HealthCheck:       upstream.Spec.HealthCheck,
		},
		CreatedAt: createdAt(upstream.ObjectMeta),
	}
}

func modelConfig(value *resource.ModelSpec) *ModelConfig {
	if value == nil {
		return nil
	}
	return &ModelConfig{
		Provider:    value.Provider,
		APIBasePath: value.APIBasePath,
		Models: lo.Map(value.Models, func(model resource.ModelCatalogItem, _ int) ModelCatalogItem {
			return ModelCatalogItem{
				Name:        model.Name,
				DisplayName: model.DisplayName,
				Enabled:     model.Enabled,
			}
		}),
	}
}

func upstreamTLS(value *resource.UpstreamTLS) *UpstreamTLS {
	if value == nil {
		return nil
	}
	return &UpstreamTLS{ServerName: value.ServerName}
}

func upstreamName(upstream *resource.Upstream) string {
	if upstream.Spec.DisplayName != "" {
		return upstream.Spec.DisplayName
	}
	return upstream.Name
}

func loadBalancePolicy(value resource.UpstreamLoadBalancePolicy) resource.UpstreamLoadBalancePolicy {
	switch value {
	case resource.UpstreamLoadBalancePolicyLeastRequest:
		return resource.UpstreamLoadBalancePolicyLeastRequest
	case resource.UpstreamLoadBalancePolicyRandom:
		return resource.UpstreamLoadBalancePolicyRandom
	default:
		return resource.UpstreamLoadBalancePolicyRoundRobin
	}
}

func endpointRequests(upstream *resource.Upstream) []UpstreamEndpoint {
	return lo.Map(upstream.Spec.Endpoints, func(endpoint resource.Endpoint, _ int) UpstreamEndpoint {
		return UpstreamEndpoint{
			ID:      endpoint.Name,
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  endpoint.Weight,
			Enabled: endpoint.Enabled,
		}
	})
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
