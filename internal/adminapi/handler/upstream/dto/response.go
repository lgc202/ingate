package dto

import (
	"time"

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
		UpstreamConfig: UpstreamConfig{
			Name:              upstreamName(upstream),
			Type:              serviceType(upstream.Spec.Type),
			Endpoints:         endpointRequests(upstream),
			LoadBalancePolicy: loadBalancePolicy(upstream.Spec.LoadBalancePolicy),
			HealthCheck:       upstream.Spec.HealthCheck,
		},
		HealthStatus:  healthStatus(upstream.Status),
		RuntimeStatus: runtimeStatus(),
		CreatedAt:     createdAt(upstream.ObjectMeta),
	}
}

func upstreamName(upstream *resource.Upstream) string {
	if upstream.Spec.DisplayName != "" {
		return upstream.Spec.DisplayName
	}
	return upstream.Name
}

func serviceType(value resource.UpstreamType) resource.UpstreamType {
	switch value {
	case resource.UpstreamTypeModel:
		return resource.UpstreamTypeModel
	case resource.UpstreamTypeAgent:
		return resource.UpstreamTypeAgent
	case resource.UpstreamTypeMCP:
		return resource.UpstreamTypeMCP
	default:
		return resource.UpstreamTypeApplication
	}
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
		weight := endpoint.Weight
		if weight == 0 {
			weight = 100
		}
		return UpstreamEndpoint{
			ID:      endpoint.Name,
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  weight,
			Enabled: endpoint.Enabled,
		}
	})
}

func healthStatus(status resource.ResourceStatus) string {
	for _, condition := range status.Conditions {
		if condition.Type == "Ready" && condition.Status == metav1.ConditionFalse {
			return "critical"
		}
		if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
			return "healthy"
		}
	}
	return "unknown"
}

func runtimeStatus() string {
	// RuntimeSnapshot 不能证明运行时已经应用，服务页先统一展示 unknown
	return "unknown"
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
