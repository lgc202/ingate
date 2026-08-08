package upstream

import (
	"net/url"
	"path"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// upstreamSpec 校验请求自身语义并转换为声明式 Upstream 配置
func upstreamSpec(
	name string,
	upstreamType string,
	protocol string,
	tls *adminv1.UpstreamTLS,
	model *adminv1.ModelConfig,
	endpoints []*adminv1.UpstreamEndpoint,
	loadBalancePolicy string,
	healthCheck *adminv1.UpstreamHealthCheck,
	apiKey *adminv1.APIKeyConfig,
) (resource.UpstreamSpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.UpstreamSpec{}, adminservice.BadRequest("服务名称不能为空")
	}
	kind := resource.UpstreamType(upstreamType)
	switch kind {
	case resource.UpstreamTypeApplication, resource.UpstreamTypeModel, resource.UpstreamTypeAgent, resource.UpstreamTypeMCP:
	default:
		return resource.UpstreamSpec{}, adminservice.BadRequest("服务类型不正确")
	}
	upstreamProtocol := resource.UpstreamProtocol(protocol)
	if !upstreamProtocol.IsSupported() {
		return resource.UpstreamSpec{}, adminservice.BadRequest("服务协议不正确")
	}

	result := resource.UpstreamSpec{
		DisplayName:       name,
		Type:              kind,
		Protocol:          upstreamProtocol,
		LoadBalancePolicy: resource.UpstreamLoadBalancePolicy(loadBalancePolicy),
	}
	switch result.LoadBalancePolicy {
	case resource.UpstreamLoadBalancePolicyRoundRobin, resource.UpstreamLoadBalancePolicyLeastRequest, resource.UpstreamLoadBalancePolicyRandom:
	default:
		return resource.UpstreamSpec{}, adminservice.BadRequest("负载均衡方式不正确")
	}
	if tls != nil {
		serverName := strings.ToLower(strings.TrimSpace(tls.GetServerName()))
		if !adminservice.ValidEndpointAddress(serverName) {
			return resource.UpstreamSpec{}, adminservice.BadRequest("HTTPS 服务名称格式不正确")
		}
		result.TLS = &resource.UpstreamTLS{ServerName: serverName}
	}
	if kind == resource.UpstreamTypeModel {
		modelSpec, err := modelSpec(model, upstreamProtocol)
		if err != nil {
			return resource.UpstreamSpec{}, err
		}
		result.Model = modelSpec
	} else if model != nil || upstreamProtocol != resource.UpstreamProtocolHTTP {
		return resource.UpstreamSpec{}, adminservice.BadRequest("非大模型服务必须使用 HTTP 协议且不能配置模型目录")
	}

	if apiKey != nil {
		if apiKey.GetValue() == "" || !httpheader.ValidValue(apiKey.GetValue()) {
			return resource.UpstreamSpec{}, adminservice.BadRequest("API Key 不能为空且不能包含非法 Header 字符")
		}
		if kind != resource.UpstreamTypeModel || result.TLS == nil {
			return resource.UpstreamSpec{}, adminservice.BadRequest("只有使用 HTTPS 的大模型服务可以配置 API Key")
		}
		result.Authentication = &resource.UpstreamAuthentication{
			APIKey: &resource.APIKeyAuthentication{Value: apiKey.GetValue()},
		}
	}
	if len(endpoints) == 0 {
		return resource.UpstreamSpec{}, adminservice.BadRequest("至少需要配置一个服务端点")
	}
	seen := make(map[string]struct{}, len(endpoints))
	enabled := 0
	for _, endpoint := range endpoints {
		if endpoint == nil {
			return resource.UpstreamSpec{}, adminservice.BadRequest("服务端点不能为空")
		}
		id := strings.TrimSpace(endpoint.GetId())
		address := strings.TrimSpace(endpoint.GetAddress())
		if id == "" || address == "" || !adminservice.ValidEndpointAddress(address) {
			return resource.UpstreamSpec{}, adminservice.BadRequest("服务端点 ID 或地址格式不正确")
		}
		if _, exists := seen[id]; exists {
			return resource.UpstreamSpec{}, adminservice.BadRequest("服务端点 ID 不能重复")
		}
		seen[id] = struct{}{}
		if endpoint.GetPort() < 1 || endpoint.GetPort() > 65535 {
			return resource.UpstreamSpec{}, adminservice.BadRequest("服务端点端口必须在 1-65535 之间")
		}
		if endpoint.GetWeight() < 1 || endpoint.GetWeight() > 100 {
			return resource.UpstreamSpec{}, adminservice.BadRequest("服务端点权重必须在 1-100 之间")
		}
		if endpoint.GetEnabled() {
			enabled++
		}
		result.Endpoints = append(result.Endpoints, resource.Endpoint{
			Name: id, Address: address, Port: int(endpoint.GetPort()), Weight: int(endpoint.GetWeight()), Enabled: endpoint.GetEnabled(),
		})
	}
	if enabled == 0 {
		return resource.UpstreamSpec{}, adminservice.BadRequest("至少需要启用一个服务端点")
	}
	if healthCheck != nil {
		result.HealthCheck = &resource.UpstreamHealthCheck{
			Enabled: healthCheck.GetEnabled(), Path: healthCheck.GetPath(),
			IntervalSeconds: int(healthCheck.GetIntervalSeconds()), TimeoutSeconds: int(healthCheck.GetTimeoutSeconds()),
		}
		if result.HealthCheck.Enabled && (!strings.HasPrefix(result.HealthCheck.Path, "/") ||
			result.HealthCheck.IntervalSeconds < 1 || result.HealthCheck.IntervalSeconds > 300 ||
			result.HealthCheck.TimeoutSeconds < 1 || result.HealthCheck.TimeoutSeconds > 60 ||
			result.HealthCheck.TimeoutSeconds >= result.HealthCheck.IntervalSeconds) {
			return resource.UpstreamSpec{}, adminservice.BadRequest("健康检查配置不正确")
		}
	}
	return result, nil
}

func modelSpec(input *adminv1.ModelConfig, protocol resource.UpstreamProtocol) (*resource.ModelSpec, error) {
	if input == nil {
		return nil, adminservice.BadRequest("大模型服务必须配置厂商和模型目录")
	}
	provider := resource.ModelProvider(input.GetProvider())
	providerProtocol, ok := provider.Protocol()
	if !ok || providerProtocol != protocol {
		return nil, adminservice.BadRequest("模型服务协议与厂商不匹配")
	}
	basePath := strings.TrimSpace(input.GetApiBasePath())
	parsed, err := url.Parse(basePath)
	if err != nil || basePath == "" || !strings.HasPrefix(basePath, "/") ||
		(basePath != "/" && strings.HasSuffix(basePath, "/")) ||
		parsed.Scheme != "" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
		parsed.Path != basePath || path.Clean(basePath) != basePath {
		return nil, adminservice.BadRequest("API 基础路径格式不正确")
	}
	if len(input.GetModels()) == 0 {
		return nil, adminservice.BadRequest("至少需要配置一个厂商模型")
	}
	result := &resource.ModelSpec{Provider: provider, APIBasePath: basePath}
	seen := make(map[string]struct{}, len(input.GetModels()))
	enabled := 0
	for _, inputModel := range input.GetModels() {
		if inputModel == nil {
			return nil, adminservice.BadRequest("厂商模型不能为空")
		}
		name := strings.TrimSpace(inputModel.GetName())
		displayName := strings.TrimSpace(inputModel.GetDisplayName())
		if name == "" || displayName == "" {
			return nil, adminservice.BadRequest("厂商模型名称和展示名称不能为空")
		}
		if _, exists := seen[name]; exists {
			return nil, adminservice.BadRequest("厂商模型名称不能重复")
		}
		seen[name] = struct{}{}
		if inputModel.GetEnabled() {
			enabled++
		}
		result.Models = append(result.Models, resource.ModelCatalogItem{
			Name: name, DisplayName: displayName, Enabled: inputModel.GetEnabled(),
		})
	}
	if enabled == 0 {
		return nil, adminservice.BadRequest("至少需要启用一个厂商模型")
	}
	return result, nil
}
