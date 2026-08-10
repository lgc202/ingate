package upstream

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultEndpointWeight             = 1
	defaultHealthCheckIntervalSeconds = 10
	defaultHealthCheckTimeoutSeconds  = 2
)

type upstreamInput struct {
	name          string
	upstreamType  adminv1.UpstreamType
	endpoints     []*adminv1.UpstreamEndpoint
	tls           *adminv1.UpstreamTLS
	loadBalancing adminv1.LoadBalancingPolicy
	healthCheck   *adminv1.UpstreamHealthCheck
	model         *adminv1.ModelConfig
	apiKey        *string
}

// buildUpstreamSpec 校验请求自身语义并构造声明式 Upstream 配置
func buildUpstreamSpec(input upstreamInput) (resource.UpstreamSpec, error) {
	name := strings.TrimSpace(input.name)
	if name == "" {
		return resource.UpstreamSpec{}, adminservice.BadRequest("服务名称不能为空")
	}
	upstreamType, err := upstreamType(input.upstreamType)
	if err != nil {
		return resource.UpstreamSpec{}, err
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
		Type:          upstreamType,
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
	if upstreamType == resource.UpstreamTypeModel {
		spec.Model, err = buildModelSpec(input.model)
		if err != nil {
			return resource.UpstreamSpec{}, err
		}
	} else if input.model != nil {
		return resource.UpstreamSpec{}, adminservice.BadRequest("只有模型服务可以配置模型厂商和目录")
	}
	if input.apiKey != nil && *input.apiKey != "" {
		if spec.Model == nil {
			return resource.UpstreamSpec{}, adminservice.BadRequest("只有模型服务可以配置 API Key")
		}
		if !httpheader.ValidValue(*input.apiKey) {
			return resource.UpstreamSpec{}, adminservice.BadRequest("API Key 不能包含非法 Header 字符")
		}
		if spec.TLS == nil {
			return resource.UpstreamSpec{}, adminservice.BadRequest("配置 API Key 时必须启用 HTTPS")
		}
		spec.Model.APIKey = *input.apiKey
	}
	return spec, nil
}

func upstreamType(value adminv1.UpstreamType) (resource.UpstreamType, error) {
	switch value {
	case adminv1.UpstreamType_UPSTREAM_TYPE_APPLICATION:
		return resource.UpstreamTypeApplication, nil
	case adminv1.UpstreamType_UPSTREAM_TYPE_MODEL:
		return resource.UpstreamTypeModel, nil
	case adminv1.UpstreamType_UPSTREAM_TYPE_AGENT:
		return resource.UpstreamTypeAgent, nil
	case adminv1.UpstreamType_UPSTREAM_TYPE_MCP:
		return resource.UpstreamTypeMCP, nil
	default:
		return "", adminservice.BadRequest("服务类型不正确")
	}
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
