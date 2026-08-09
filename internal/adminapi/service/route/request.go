package route

import (
	"net/http"
	"strings"

	"golang.org/x/net/http/httpguts"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultRouteTimeoutMillis = 30000
	openAIChatCompletionsPath = "/v1/chat/completions"
	aiClusterHeader           = "x-ingate-ai-cluster-v1"
)

var aiManagedRequestHeaders = map[string]struct{}{
	":authority":        {},
	":path":             {},
	"accept-encoding":   {},
	"anthropic-version": {},
	"authorization":     {},
	"content-encoding":  {},
	"content-length":    {},
	"content-type":      {},
	aiClusterHeader:     {},
	"x-api-key":         {},
	"x-goog-api-key":    {},
}

// buildRouteSpec 校验请求自身语义并构造声明式 Route 配置
func buildRouteSpec(
	name string,
	gatewayIDs []string,
	hostnames []string,
	enabled *bool,
	inputRules []*adminv1.RouteRule,
) (resource.RouteSpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.RouteSpec{}, adminservice.BadRequest("路由名称不能为空")
	}
	if len(gatewayIDs) == 0 {
		return resource.RouteSpec{}, adminservice.BadRequest("至少需要选择一个网关")
	}
	spec := resource.RouteSpec{DisplayName: name, Enabled: enabled == nil || *enabled}
	for _, id := range gatewayIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return resource.RouteSpec{}, adminservice.BadRequest("网关 ID 不能为空")
		}
		spec.ParentRefs = append(spec.ParentRefs, resource.ParentRef{Name: id})
	}
	for _, hostname := range hostnames {
		hostname = strings.ToLower(strings.TrimSpace(hostname))
		if !validRouteHostname(hostname) {
			return resource.RouteSpec{}, adminservice.BadRequest("路由 Host 格式不正确")
		}
		spec.Hostnames = append(spec.Hostnames, hostname)
	}
	if len(inputRules) == 0 {
		return resource.RouteSpec{}, adminservice.BadRequest("至少需要一条路由规则")
	}
	seen := make(map[string]struct{}, len(inputRules))
	for _, input := range inputRules {
		rule, err := buildRouteRule(input)
		if err != nil {
			return resource.RouteSpec{}, err
		}
		if _, exists := seen[rule.Name]; exists {
			return resource.RouteSpec{}, adminservice.BadRequest("路由规则名称不能重复")
		}
		seen[rule.Name] = struct{}{}
		spec.Rules = append(spec.Rules, rule)
	}
	return spec, nil
}

func buildRouteRule(input *adminv1.RouteRule) (resource.RouteRule, error) {
	if input == nil {
		return resource.RouteRule{}, adminservice.BadRequest("路由规则不能为空")
	}
	rule := resource.RouteRule{Name: strings.TrimSpace(input.GetName()), PathPrefix: strings.TrimSpace(input.GetPathPrefix())}
	if rule.Name == "" {
		return resource.RouteRule{}, adminservice.BadRequest("路由规则名称不能为空")
	}
	if !strings.HasPrefix(rule.PathPrefix, "/") {
		return resource.RouteRule{}, adminservice.BadRequest("路由规则路径前缀必须以 / 开头")
	}
	for _, method := range input.GetMethods() {
		if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut &&
			method != http.MethodPatch && method != http.MethodDelete {
			return resource.RouteRule{}, adminservice.BadRequest("路由规则包含不支持的 HTTP 方法")
		}
		rule.Methods = append(rule.Methods, method)
	}
	seenHeaders := make(map[string]struct{}, len(input.GetHeaders()))
	for _, header := range input.GetHeaders() {
		if header == nil {
			return resource.RouteRule{}, adminservice.BadRequest("路由规则 Header 名称和值不能为空")
		}
		name := strings.ToLower(strings.TrimSpace(header.GetName()))
		value := header.GetValue()
		if !httpguts.ValidHeaderFieldName(name) || value == "" || !httpguts.ValidHeaderFieldValue(value) {
			return resource.RouteRule{}, adminservice.BadRequest("路由规则 Header 名称或值格式不正确")
		}
		if _, exists := seenHeaders[name]; exists {
			return resource.RouteRule{}, adminservice.BadRequest("同一条路由规则不能重复匹配相同 Header")
		}
		seenHeaders[name] = struct{}{}
		if input.GetModelRouting() != nil && name == aiClusterHeader {
			return resource.RouteRule{}, adminservice.BadRequest("模型路由不能匹配系统内部 Header")
		}
		rule.Headers = append(rule.Headers, resource.HeaderMatch{Name: name, Value: value})
	}

	if input.GetModelRouting() != nil {
		if len(input.GetTargets()) > 0 || rule.PathPrefix != openAIChatCompletionsPath ||
			len(rule.Methods) != 1 || rule.Methods[0] != http.MethodPost {
			return resource.RouteRule{}, adminservice.BadRequest("模型路由必须仅使用 POST /v1/chat/completions 且不能配置普通目标")
		}
		modelRouting := &resource.ModelRouting{}
		seenModels := make(map[string]struct{}, len(input.GetModelRouting().GetModels()))
		for _, model := range input.GetModelRouting().GetModels() {
			if model == nil {
				return resource.RouteRule{}, adminservice.BadRequest("模型路由配置不能为空")
			}
			name := strings.TrimSpace(model.GetModel())
			upstreamID := strings.TrimSpace(model.GetUpstreamId())
			if name == "" || upstreamID == "" {
				return resource.RouteRule{}, adminservice.BadRequest("客户端模型名称和模型服务不能为空")
			}
			if _, exists := seenModels[name]; exists {
				return resource.RouteRule{}, adminservice.BadRequest("客户端模型名称不能重复")
			}
			seenModels[name] = struct{}{}
			modelRouting.Models = append(modelRouting.Models, resource.ModelRoute{
				Model: name, UpstreamRef: upstreamID, UpstreamModel: strings.TrimSpace(model.GetUpstreamModel()),
			})
		}
		if len(modelRouting.Models) == 0 {
			return resource.RouteRule{}, adminservice.BadRequest("至少需要配置一个模型")
		}
		rule.ModelRouting = modelRouting
	} else {
		if len(input.GetTargets()) == 0 {
			return resource.RouteRule{}, adminservice.BadRequest("至少需要选择一个目标服务")
		}
		seenTargets := make(map[string]struct{}, len(input.GetTargets()))
		for _, target := range input.GetTargets() {
			if target == nil || strings.TrimSpace(target.GetUpstreamId()) == "" ||
				target.GetWeight() < 1 || target.GetWeight() > 100 {
				return resource.RouteRule{}, adminservice.BadRequest("目标服务配置不正确")
			}
			id := strings.TrimSpace(target.GetUpstreamId())
			if _, exists := seenTargets[id]; exists {
				return resource.RouteRule{}, adminservice.BadRequest("目标服务不能重复")
			}
			seenTargets[id] = struct{}{}
			rule.UpstreamRefs = append(rule.UpstreamRefs, resource.UpstreamRef{Name: id, Weight: int(target.GetWeight())})
		}
	}
	if input.GetRequestHeaderModifier() != nil {
		modifier, err := buildHeaderModifier(input.GetRequestHeaderModifier())
		if err != nil {
			return resource.RouteRule{}, err
		}
		if rule.ModelRouting != nil && containsManagedHeader(modifier) {
			return resource.RouteRule{}, adminservice.BadRequest("模型路由的请求 Header 改写不能使用系统管理的名称")
		}
		rule.Filters = append(rule.Filters, resource.RouteFilter{
			Type: resource.RouteFilterRequestHeaderModifier, RequestHeaderModifier: modifier,
		})
	}
	if input.GetResponseHeaderModifier() != nil {
		modifier, err := buildHeaderModifier(input.GetResponseHeaderModifier())
		if err != nil {
			return resource.RouteRule{}, err
		}
		rule.Filters = append(rule.Filters, resource.RouteFilter{
			Type: resource.RouteFilterResponseHeaderModifier, ResponseHeaderModifier: modifier,
		})
	}
	totalTimeoutMillis := int32(defaultRouteTimeoutMillis)
	if timeout := input.GetTimeout(); timeout != nil {
		if timeout.GetRequestMillis() < 100 || timeout.GetRequestMillis() > 300000 {
			return resource.RouteRule{}, adminservice.BadRequest("请求超时必须在 100-300000 毫秒之间")
		}
		totalTimeoutMillis = timeout.GetRequestMillis()
		rule.Timeout = &resource.RouteTimeout{RequestMillis: int(timeout.GetRequestMillis())}
	}
	if retry := input.GetRetry(); retry != nil {
		if rule.ModelRouting != nil || retry.GetAttempts() < 1 || retry.GetAttempts() > 5 ||
			retry.GetPerTryTimeoutMillis() < 100 || retry.GetPerTryTimeoutMillis() > 60000 {
			return resource.RouteRule{}, adminservice.BadRequest("重试配置不正确")
		}
		if retry.GetPerTryTimeoutMillis() > totalTimeoutMillis {
			return resource.RouteRule{}, adminservice.BadRequest("单次重试超时不能大于请求总超时")
		}
		rule.Retry = &resource.RouteRetry{
			Attempts: int(retry.GetAttempts()), PerTryTimeoutMillis: int(retry.GetPerTryTimeoutMillis()),
		}
	}
	return rule, nil
}

func containsManagedHeader(modifier *resource.HeaderModifier) bool {
	for _, header := range modifier.Set {
		if _, exists := aiManagedRequestHeaders[header.Name]; exists {
			return true
		}
	}
	for _, name := range modifier.Remove {
		if _, exists := aiManagedRequestHeaders[name]; exists {
			return true
		}
	}
	return false
}

func buildHeaderModifier(input *adminv1.HeaderModifier) (*resource.HeaderModifier, error) {
	modifier := &resource.HeaderModifier{}
	seen := make(map[string]string, len(input.GetSet())+len(input.GetRemove()))
	for _, header := range input.GetSet() {
		if header == nil {
			return nil, adminservice.BadRequest("Header 名称和值不能为空")
		}
		name := strings.ToLower(strings.TrimSpace(header.GetName()))
		value := header.GetValue()
		if !httpguts.ValidHeaderFieldName(name) || value == "" || !httpguts.ValidHeaderFieldValue(value) {
			return nil, adminservice.BadRequest("Header 名称或值格式不正确")
		}
		if _, exists := seen[name]; exists {
			return nil, adminservice.BadRequest("同一个 Header 只能配置一次")
		}
		seen[name] = "set"
		modifier.Set = append(modifier.Set, resource.HeaderValue{
			Name: name, Value: value,
		})
	}
	for _, name := range input.GetRemove() {
		name = strings.ToLower(strings.TrimSpace(name))
		if !httpguts.ValidHeaderFieldName(name) {
			return nil, adminservice.BadRequest("待删除的 Header 名称格式不正确")
		}
		if _, exists := seen[name]; exists {
			return nil, adminservice.BadRequest("同一个 Header 不能同时写入和删除")
		}
		seen[name] = "remove"
		modifier.Remove = append(modifier.Remove, name)
	}
	if len(modifier.Set) == 0 && len(modifier.Remove) == 0 {
		return nil, adminservice.BadRequest("至少需要配置一个 Header 写入或删除动作")
	}
	return modifier, nil
}

func validRouteHostname(hostname string) bool {
	if strings.HasPrefix(hostname, "*.") {
		hostname = strings.TrimPrefix(hostname, "*.")
	}
	return adminservice.ValidEndpointAddress(hostname)
}
