package upstream

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultEndpointWeight             = 1
	defaultHealthCheckIntervalSeconds = 10
	defaultHealthCheckTimeoutSeconds  = 2
)

type upstreamInput struct {
	name          string
	endpoints     []*adminv1.UpstreamEndpoint
	tls           *adminv1.UpstreamTLS
	loadBalancing adminv1.LoadBalancingPolicy
	healthCheck   *adminv1.UpstreamHealthCheck
}

// buildUpstreamSpec 校验请求自身语义并构造声明式 Upstream 配置
func buildUpstreamSpec(input upstreamInput) (resource.UpstreamSpec, error) {
	name := strings.TrimSpace(input.name)
	if name == "" {
		return resource.UpstreamSpec{}, adminservice.BadRequest("服务名称不能为空")
	}
	endpoints, err := buildEndpoints(input.endpoints)
	if err != nil {
		return resource.UpstreamSpec{}, err
	}
	loadBalancing, err := loadBalancingPolicy(input.loadBalancing)
	if err != nil {
		return resource.UpstreamSpec{}, err
	}

	spec := resource.UpstreamSpec{
		DisplayName:   name,
		Endpoints:     endpoints,
		LoadBalancing: loadBalancing,
	}
	if input.tls != nil {
		serverName := strings.ToLower(strings.TrimSpace(input.tls.GetServerName()))
		if !adminservice.ValidEndpointAddress(serverName) {
			return resource.UpstreamSpec{}, adminservice.BadRequest("HTTPS 服务名称格式不正确")
		}
		spec.TLS = &resource.UpstreamTLS{ServerName: serverName}
	}
	if input.healthCheck != nil {
		spec.HealthCheck, err = buildHealthCheck(input.healthCheck)
		if err != nil {
			return resource.UpstreamSpec{}, err
		}
	}
	return spec, nil
}

func loadBalancingPolicy(value adminv1.LoadBalancingPolicy) (resource.LoadBalancingPolicy, error) {
	switch value {
	case adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_UNSPECIFIED,
		adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_ROUND_ROBIN:
		return resource.LoadBalancingRoundRobin, nil
	case adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_LEAST_REQUEST:
		return resource.LoadBalancingLeastRequest, nil
	default:
		return "", adminservice.BadRequest("负载均衡方式不正确")
	}
}
