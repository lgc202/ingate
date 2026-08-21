package upstream

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

const (
	defaultEndpointWeight             = 1
	defaultHealthCheckIntervalSeconds = 10
	defaultHealthCheckTimeoutSeconds  = 2
)

func createSpec(request *adminv1.CreateUpstreamRequest) (resource.UpstreamSpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.UpstreamSpec{}, adminservice.BadRequest("服务名称不能为空")
	}
	serviceEndpoints, err := upstreamEndpoints(request.GetEndpoints())
	if err != nil {
		return resource.UpstreamSpec{}, err
	}
	loadBalancing, err := loadBalancingPolicy(request.GetLoadBalancing())
	if err != nil {
		return resource.UpstreamSpec{}, err
	}
	tls, err := upstreamTLS(request.GetTls())
	if err != nil {
		return resource.UpstreamSpec{}, err
	}
	check, err := upstreamHealthCheck(request.GetHealthCheck())
	if err != nil {
		return resource.UpstreamSpec{}, err
	}
	model, err := modelForCreate(request.GetModel())
	if err != nil {
		return resource.UpstreamSpec{}, err
	}
	return resource.UpstreamSpec{
		DisplayName:   name,
		Endpoints:     serviceEndpoints,
		TLS:           tls,
		LoadBalancing: loadBalancing,
		HealthCheck:   check,
		Model:         model,
	}, nil
}

// updateSpec 额外返回是否保留已有 API Key，避免把“未填写”误解为“清空”
func updateSpec(request *adminv1.UpdateUpstreamRequest) (resource.UpstreamSpec, bool, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.UpstreamSpec{}, false, adminservice.BadRequest("服务名称不能为空")
	}
	serviceEndpoints, err := upstreamEndpoints(request.GetEndpoints())
	if err != nil {
		return resource.UpstreamSpec{}, false, err
	}
	loadBalancing, err := loadBalancingPolicy(request.GetLoadBalancing())
	if err != nil {
		return resource.UpstreamSpec{}, false, err
	}
	tls, err := upstreamTLS(request.GetTls())
	if err != nil {
		return resource.UpstreamSpec{}, false, err
	}
	check, err := upstreamHealthCheck(request.GetHealthCheck())
	if err != nil {
		return resource.UpstreamSpec{}, false, err
	}
	model, preserveAPIKey, err := modelForUpdate(request.GetModel())
	if err != nil {
		return resource.UpstreamSpec{}, false, err
	}
	return resource.UpstreamSpec{
		DisplayName:   name,
		Endpoints:     serviceEndpoints,
		TLS:           tls,
		LoadBalancing: loadBalancing,
		HealthCheck:   check,
		Model:         model,
	}, preserveAPIKey, nil
}

func upstreamTLS(input *adminv1.UpstreamTLS) (*resource.UpstreamTLS, error) {
	if input == nil {
		return nil, nil
	}
	serverName := strings.ToLower(strings.TrimSpace(input.GetServerName()))
	if !validEndpointAddress(serverName) {
		return nil, adminservice.BadRequest("HTTPS 服务名称格式不正确")
	}
	return &resource.UpstreamTLS{ServerName: serverName}, nil
}

func modelForCreate(input *adminv1.ModelUpstreamInput) (*resource.ModelUpstream, error) {
	if input == nil {
		return nil, nil
	}
	if input.GetClearApiKey() {
		return nil, adminservice.BadRequest("创建模型服务时不能清除 API Key")
	}
	model, _, err := modelUpstream(input)
	return model, err
}

func modelForUpdate(input *adminv1.ModelUpstreamInput) (*resource.ModelUpstream, bool, error) {
	if input == nil {
		return nil, false, nil
	}
	return modelUpstream(input)
}

func modelUpstream(input *adminv1.ModelUpstreamInput) (*resource.ModelUpstream, bool, error) {
	if input.ApiKey != nil && input.GetClearApiKey() {
		return nil, false, adminservice.BadRequest("不能同时填写新 API Key 和清除已有 API Key")
	}

	protocol := resource.ModelProtocol("")
	switch input.GetProtocol() {
	case adminv1.ModelProtocol_MODEL_PROTOCOL_OPENAI:
		protocol = resource.ModelProtocolOpenAI
	case adminv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC:
		protocol = resource.ModelProtocolAnthropic
	default:
		return nil, false, adminservice.BadRequest("请选择模型服务协议")
	}

	model := &resource.ModelUpstream{Protocol: protocol}
	if input.ApiKey == nil {
		return model, !input.GetClearApiKey(), nil
	}
	if input.GetApiKey() == "" || strings.TrimSpace(input.GetApiKey()) != input.GetApiKey() {
		return nil, false, adminservice.BadRequest("API Key 不能为空或包含首尾空格")
	}
	model.APIKey = input.GetApiKey()
	return model, false, nil
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
