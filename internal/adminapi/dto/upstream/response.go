package upstream

import (
	"time"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewListUpstreamsResp 转换 Upstream 列表用例结果为控制台服务列表响应
func NewListUpstreamsResp(result *upstreamservice.ListResult) ListUpstreamsResp {
	return ListUpstreamsResp{
		Upstreams: lo.Map(result.Upstreams, func(upstream resource.Upstream, _ int) Upstream {
			return upstreamFromResource(&upstream)
		}),
	}
}

// NewGetUpstreamResp 转换单个 Upstream 用例结果为控制台服务响应
func NewGetUpstreamResp(result *upstreamservice.UpstreamResult) Upstream {
	return upstreamFromResource(result.Upstream)
}

func upstreamFromResource(upstream *resource.Upstream) Upstream {
	return Upstream{
		ID:      upstream.Name,
		Version: upstream.ResourceVersion,
		Status:  admindto.NewResourceStatus(upstream.Generation, upstream.Status.Conditions),
		UpstreamConfig: UpstreamConfig{
			Name:              upstreamName(upstream),
			Type:              upstream.Spec.Type,
			Protocol:          upstream.Spec.Protocol,
			TLS:               upstreamTLS(upstream.Spec.TLS),
			CredentialID:      upstream.Spec.CredentialRef,
			Endpoints:         endpointRequests(upstream),
			LoadBalancePolicy: loadBalancePolicy(upstream.Spec.LoadBalancePolicy),
			HealthCheck:       upstream.Spec.HealthCheck,
		},
		CreatedAt: createdAt(upstream.ObjectMeta),
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
