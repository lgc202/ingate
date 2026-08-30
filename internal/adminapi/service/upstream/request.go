package upstream

import (
	"strings"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
)

func parseUpstreamSpec(
	displayName string,
	endpointConfigs []*adminv1.UpstreamEndpoint,
	tlsConfig *adminv1.UpstreamTLS,
	loadBalancingConfig adminv1.LoadBalancingPolicy,
	healthCheckConfig *adminv1.UpstreamHealthCheck,
) (resource.UpstreamSpec, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return resource.UpstreamSpec{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"服务名称不能为空",
		)
	}
	endpoints, err := parseEndpoints(endpointConfigs)
	if err != nil {
		return resource.UpstreamSpec{}, err
	}
	loadBalancing, err := parseLoadBalancingPolicy(loadBalancingConfig)
	if err != nil {
		return resource.UpstreamSpec{}, err
	}
	tls, err := parseTLS(tlsConfig)
	if err != nil {
		return resource.UpstreamSpec{}, err
	}
	healthCheck, err := parseHealthCheck(healthCheckConfig)
	if err != nil {
		return resource.UpstreamSpec{}, err
	}
	return resource.UpstreamSpec{
		DisplayName:   displayName,
		Endpoints:     endpoints,
		TLS:           tls,
		LoadBalancing: loadBalancing,
		HealthCheck:   healthCheck,
	}, nil
}

func parseTLS(config *adminv1.UpstreamTLS) (*resource.UpstreamTLS, error) {
	if config == nil {
		return nil, nil
	}
	serverName := upstreamconfig.NormalizeAddress(config.GetServerName())
	if !upstreamconfig.IsValidAddress(serverName) {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"HTTPS 服务名称格式不正确",
		)
	}
	return &resource.UpstreamTLS{ServerName: serverName}, nil
}

func parseLoadBalancingPolicy(
	config adminv1.LoadBalancingPolicy,
) (resource.LoadBalancingPolicy, error) {
	switch config {
	case adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_UNSPECIFIED,
		adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_ROUND_ROBIN:
		return resource.LoadBalancingRoundRobin, nil
	case adminv1.LoadBalancingPolicy_LOAD_BALANCING_POLICY_LEAST_REQUEST:
		return resource.LoadBalancingLeastRequest, nil
	default:
		return "", errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"负载均衡方式不正确",
		)
	}
}

func parseModelForCreate(config *adminv1.ModelUpstreamInput) (*resource.ModelUpstream, error) {
	if config == nil {
		return nil, nil
	}
	if config.GetClearApiKey() {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"创建模型服务时不能清除 API Key",
		)
	}
	model, _, err := parseModel(config)
	return model, err
}

func parseModelForUpdate(config *adminv1.ModelUpstreamInput) (*resource.ModelUpstream, bool, error) {
	if config == nil {
		return nil, false, nil
	}
	return parseModel(config)
}

// parseModel 返回模型配置，以及更新时是否应保留已存储的 API Key。
func parseModel(config *adminv1.ModelUpstreamInput) (*resource.ModelUpstream, bool, error) {
	if config.ApiKey != nil && config.GetClearApiKey() {
		return nil, false, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"不能同时填写新 API Key 和清除已有 API Key",
		)
	}

	var protocol resource.ModelProtocol
	switch config.GetProtocol() {
	case adminv1.ModelProtocol_MODEL_PROTOCOL_OPENAI:
		protocol = resource.ModelProtocolOpenAI
	case adminv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC:
		protocol = resource.ModelProtocolAnthropic
	default:
		return nil, false, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"请选择模型服务协议",
		)
	}

	model := &resource.ModelUpstream{Protocol: protocol}
	if config.ApiKey == nil {
		return model, !config.GetClearApiKey(), nil
	}
	apiKey := config.GetApiKey()
	if apiKey == "" || !upstreamconfig.IsValidModelAPIKey(apiKey) {
		return nil, false, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"API Key 不能为空、包含首尾空格或超过长度限制",
		)
	}
	model.APIKey = apiKey
	return model, false, nil
}
