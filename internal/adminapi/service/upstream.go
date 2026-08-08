package service

import (
	"context"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// UpstreamService 实现服务管理 API
type UpstreamService struct {
	usecase *biz.UpstreamUsecase
}

// NewUpstreamService 创建服务协议层
func NewUpstreamService(usecase *biz.UpstreamUsecase) *UpstreamService {
	return &UpstreamService{usecase: usecase}
}

func (s *UpstreamService) ListUpstreams(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListUpstreamsReply, error) {
	items, err := s.usecase.List(ctx)
	if err != nil {
		return nil, operationError(err, "查询服务失败")
	}
	reply := &adminv1.ListUpstreamsReply{Upstreams: make([]*adminv1.Upstream, 0, len(items))}
	for i := range items {
		reply.Upstreams = append(reply.Upstreams, upstreamReply(&items[i]))
	}
	return reply, nil
}

func (s *UpstreamService) GetUpstream(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.Upstream, error) {
	item, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, operationError(err, "查询服务失败")
	}
	return upstreamReply(item), nil
}

func (s *UpstreamService) CreateUpstream(ctx context.Context, request *adminv1.CreateUpstreamRequest) (*adminv1.MutationReply, error) {
	spec, err := upstreamSpec(
		request.GetName(), request.GetType(), request.GetProtocol(), request.GetTls(), request.GetModel(),
		request.GetEndpoints(), request.GetLoadBalancePolicy(), request.GetHealthCheck(), request.GetApiKey(),
	)
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, operationError(err, "创建服务失败")
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *UpstreamService) UpdateUpstream(ctx context.Context, request *adminv1.UpdateUpstreamRequest) (*adminv1.MutationReply, error) {
	if request.GetApiKey() != nil && request.GetRemoveApiKey() {
		return nil, badRequest("不能同时设置和移除 API Key")
	}
	spec, err := upstreamSpec(
		request.GetName(), request.GetType(), request.GetProtocol(), request.GetTls(), request.GetModel(),
		request.GetEndpoints(), request.GetLoadBalancePolicy(), request.GetHealthCheck(), request.GetApiKey(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec, request.GetRemoveApiKey()); err != nil {
		return nil, operationError(err, "更新服务失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *UpstreamService) DeleteUpstream(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, operationError(err, "删除服务失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

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
		return resource.UpstreamSpec{}, badRequest("服务名称不能为空")
	}
	kind := resource.UpstreamType(upstreamType)
	switch kind {
	case resource.UpstreamTypeApplication, resource.UpstreamTypeModel, resource.UpstreamTypeAgent, resource.UpstreamTypeMCP:
	default:
		return resource.UpstreamSpec{}, badRequest("服务类型不正确")
	}
	upstreamProtocol := resource.UpstreamProtocol(protocol)
	if !upstreamProtocol.IsSupported() {
		return resource.UpstreamSpec{}, badRequest("服务协议不正确")
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
		return resource.UpstreamSpec{}, badRequest("负载均衡方式不正确")
	}
	if tls != nil {
		serverName := strings.ToLower(strings.TrimSpace(tls.GetServerName()))
		if !validEndpointAddress(serverName) {
			return resource.UpstreamSpec{}, badRequest("HTTPS 服务名称格式不正确")
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
		return resource.UpstreamSpec{}, badRequest("非大模型服务必须使用 HTTP 协议且不能配置模型目录")
	}

	if apiKey != nil {
		if apiKey.GetValue() == "" || !httpheader.ValidValue(apiKey.GetValue()) {
			return resource.UpstreamSpec{}, badRequest("API Key 不能为空且不能包含非法 Header 字符")
		}
		if kind != resource.UpstreamTypeModel || result.TLS == nil {
			return resource.UpstreamSpec{}, badRequest("只有使用 HTTPS 的大模型服务可以配置 API Key")
		}
		result.Authentication = &resource.UpstreamAuthentication{
			APIKey: &resource.APIKeyAuthentication{Value: apiKey.GetValue()},
		}
	}
	if len(endpoints) == 0 {
		return resource.UpstreamSpec{}, badRequest("至少需要配置一个服务端点")
	}
	seen := make(map[string]struct{}, len(endpoints))
	enabled := 0
	for _, endpoint := range endpoints {
		if endpoint == nil {
			return resource.UpstreamSpec{}, badRequest("服务端点不能为空")
		}
		id := strings.TrimSpace(endpoint.GetId())
		address := strings.TrimSpace(endpoint.GetAddress())
		if id == "" || address == "" || !validEndpointAddress(address) {
			return resource.UpstreamSpec{}, badRequest("服务端点 ID 或地址格式不正确")
		}
		if _, exists := seen[id]; exists {
			return resource.UpstreamSpec{}, badRequest("服务端点 ID 不能重复")
		}
		seen[id] = struct{}{}
		if endpoint.GetPort() < 1 || endpoint.GetPort() > 65535 {
			return resource.UpstreamSpec{}, badRequest("服务端点端口必须在 1-65535 之间")
		}
		if endpoint.GetWeight() < 1 || endpoint.GetWeight() > 100 {
			return resource.UpstreamSpec{}, badRequest("服务端点权重必须在 1-100 之间")
		}
		if endpoint.GetEnabled() {
			enabled++
		}
		result.Endpoints = append(result.Endpoints, resource.Endpoint{
			Name: id, Address: address, Port: int(endpoint.GetPort()), Weight: int(endpoint.GetWeight()), Enabled: endpoint.GetEnabled(),
		})
	}
	if enabled == 0 {
		return resource.UpstreamSpec{}, badRequest("至少需要启用一个服务端点")
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
			return resource.UpstreamSpec{}, badRequest("健康检查配置不正确")
		}
	}
	return result, nil
}

func modelSpec(input *adminv1.ModelConfig, protocol resource.UpstreamProtocol) (*resource.ModelSpec, error) {
	if input == nil {
		return nil, badRequest("大模型服务必须配置厂商和模型目录")
	}
	provider := resource.ModelProvider(input.GetProvider())
	providerProtocol, ok := provider.Protocol()
	if !ok || providerProtocol != protocol {
		return nil, badRequest("模型服务协议与厂商不匹配")
	}
	basePath := strings.TrimSpace(input.GetApiBasePath())
	parsed, err := url.Parse(basePath)
	if err != nil || basePath == "" || !strings.HasPrefix(basePath, "/") ||
		(basePath != "/" && strings.HasSuffix(basePath, "/")) ||
		parsed.Scheme != "" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
		parsed.Path != basePath || path.Clean(basePath) != basePath {
		return nil, badRequest("API 基础路径格式不正确")
	}
	if len(input.GetModels()) == 0 {
		return nil, badRequest("至少需要配置一个厂商模型")
	}
	result := &resource.ModelSpec{Provider: provider, APIBasePath: basePath}
	seen := make(map[string]struct{}, len(input.GetModels()))
	enabled := 0
	for _, inputModel := range input.GetModels() {
		if inputModel == nil {
			return nil, badRequest("厂商模型不能为空")
		}
		name := strings.TrimSpace(inputModel.GetName())
		displayName := strings.TrimSpace(inputModel.GetDisplayName())
		if name == "" || displayName == "" {
			return nil, badRequest("厂商模型名称和展示名称不能为空")
		}
		if _, exists := seen[name]; exists {
			return nil, badRequest("厂商模型名称不能重复")
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
		return nil, badRequest("至少需要启用一个厂商模型")
	}
	return result, nil
}

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

func validEndpointAddress(address string) bool {
	if _, err := netip.ParseAddr(address); err == nil {
		return true
	}
	if address == "" || len(address) > 253 {
		return false
	}
	for label := range strings.SplitSeq(strings.ToLower(address), ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}
