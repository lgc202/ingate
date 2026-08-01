package route

import (
	"errors"
	"net/http"
	"strings"
)

const (
	defaultRouteTimeoutMillis = 30000
	minRouteTimeoutMillis     = 100
	maxRouteTimeoutMillis     = 300000
	minRetryAttempts          = 1
	maxRetryAttempts          = 5
	minPerTryTimeoutMillis    = 100
	maxPerTryTimeoutMillis    = 60000
	openAIChatCompletionsPath = "/v1/chat/completions"
	aiClusterHeader           = "x-ingate-ai-cluster-v1"
	aiRouteHeader             = "x-ingate-ai-route-v1"
)

var aiManagedRequestHeaders = []string{
	":authority",
	":path",
	"accept-encoding",
	"anthropic-version",
	"authorization",
	"content-encoding",
	"content-length",
	"content-type",
	aiClusterHeader,
	aiRouteHeader,
	"x-api-key",
	"x-goog-api-key",
}

// Validate 校验创建 Route 请求
func (r *CreateRouteReq) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.New("路由名称不能为空")
	}

	gatewayIDs, err := routeGatewayIDs(r.GatewayIDs)
	if err != nil {
		return err
	}
	r.GatewayIDs = gatewayIDs

	hostnames, err := routeHostnames(r.Hostnames)
	if err != nil {
		return err
	}
	r.Hostnames = hostnames

	if len(r.Rules) == 0 {
		return errors.New("至少需要一条路由规则")
	}
	seenRules := make(map[string]struct{}, len(r.Rules))
	for i := range r.Rules {
		if err := r.Rules[i].Validate(); err != nil {
			return err
		}
		if _, ok := seenRules[r.Rules[i].Name]; ok {
			return errors.New("路由规则名称不能重复")
		}
		seenRules[r.Rules[i].Name] = struct{}{}
	}
	return nil
}

// Validate 校验更新 Route 请求
func (r *UpdateRouteReq) Validate() error {
	if r.Version == "" {
		return errors.New("路由版本不能为空")
	}
	return r.CreateRouteReq.Validate()
}

// EnabledValue 返回创建和更新请求中的 Route 启停值
func (r CreateRouteReq) EnabledValue() bool {
	return r.Enabled == nil || *r.Enabled
}

func routeGatewayIDs(items []string) ([]string, error) {
	if len(items) == 0 {
		return nil, errors.New("至少需要选择一个网关")
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item)
		if id == "" {
			return nil, errors.New("网关 ID 不能为空")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func routeHostnames(items []string) ([]string, error) {
	hostnames := make([]string, 0, len(items))
	for _, item := range items {
		hostname := strings.TrimSpace(strings.ToLower(item))
		if hostname == "" {
			return nil, errors.New("路由 Host 不能为空")
		}
		if !validHostname(hostname) {
			return nil, errors.New("路由 Host 格式不正确")
		}
		hostnames = append(hostnames, hostname)
	}
	return hostnames, nil
}

// Validate 校验并规整单条 Route 规则
func (r *RouteRule) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.New("路由规则名称不能为空")
	}

	r.PathPrefix = strings.TrimSpace(r.PathPrefix)
	if r.PathPrefix == "" || !strings.HasPrefix(r.PathPrefix, "/") {
		return errors.New("路由规则路径前缀必须以 / 开头")
	}

	for _, method := range r.Methods {
		if !validHTTPMethod(method) {
			return errors.New("路由规则包含不支持的 HTTP 方法")
		}
	}

	headers, err := r.headerMatches()
	if err != nil {
		return err
	}
	r.Headers = headers

	if r.ModelRouting == nil {
		targets, err := r.targetServices()
		if err != nil {
			return err
		}
		r.Targets = targets
	} else {
		if len(r.Targets) > 0 {
			return errors.New("普通目标服务和模型路由不能同时配置")
		}
		if r.PathPrefix != openAIChatCompletionsPath {
			return errors.New("模型路由路径必须为 /v1/chat/completions")
		}
		if err := r.ModelRouting.Validate(r.Methods); err != nil {
			return err
		}
	}
	return r.validateRouteNativePolicies()
}

// Validate 校验并规整模型路由配置
func (r *ModelRouting) Validate(methods []string) error {
	if len(methods) != 1 || methods[0] != http.MethodPost {
		return errors.New("模型路由只支持 POST 方法")
	}
	if len(r.Models) == 0 {
		return errors.New("至少需要配置一个模型")
	}

	seenModels := make(map[string]struct{}, len(r.Models))
	for i := range r.Models {
		model := &r.Models[i]
		model.Model = strings.TrimSpace(model.Model)
		model.UpstreamID = strings.TrimSpace(model.UpstreamID)
		model.UpstreamModel = strings.TrimSpace(model.UpstreamModel)
		if model.Model == "" {
			return errors.New("客户端模型名称不能为空")
		}
		if model.UpstreamID == "" {
			return errors.New("模型服务 ID 不能为空")
		}
		if _, exists := seenModels[model.Model]; exists {
			return errors.New("客户端模型名称不能重复")
		}
		seenModels[model.Model] = struct{}{}
	}
	return nil
}

func (r *RouteRule) headerMatches() ([]HeaderMatchReq, error) {
	headers := make([]HeaderMatchReq, 0, len(r.Headers))
	for _, header := range r.Headers {
		name := strings.TrimSpace(strings.ToLower(header.Name))
		value := strings.TrimSpace(header.Value)
		if name == "" {
			return nil, errors.New("路由规则 Header 名称不能为空")
		}
		if value == "" {
			return nil, errors.New("路由规则 Header 值不能为空")
		}
		if r.ModelRouting != nil && (name == aiClusterHeader || name == aiRouteHeader) {
			return nil, errors.New("模型路由不能匹配系统内部 Header")
		}
		headers = append(headers, HeaderMatchReq{Name: name, Value: value})
	}
	return headers, nil
}

func (r *RouteRule) targetServices() ([]RouteTarget, error) {
	targets := r.Targets
	if len(targets) == 0 {
		return nil, errors.New("至少需要选择一个目标服务")
	}
	seenTargets := map[string]struct{}{}
	values := make([]RouteTarget, 0, len(targets))
	for _, target := range targets {
		upstreamID := strings.TrimSpace(target.UpstreamID)
		if upstreamID == "" {
			return nil, errors.New("目标服务 ID 不能为空")
		}
		if _, ok := seenTargets[upstreamID]; ok {
			return nil, errors.New("目标服务不能重复")
		}
		if target.Weight < 1 || target.Weight > 100 {
			return nil, errors.New("目标服务权重必须在 1-100 之间")
		}
		seenTargets[upstreamID] = struct{}{}
		values = append(values, RouteTarget{UpstreamID: upstreamID, Weight: target.Weight})
	}
	return values, nil
}

func (r *RouteRule) validateRouteNativePolicies() error {
	totalTimeoutMillis := defaultRouteTimeoutMillis

	if r.RequestHeaderModifier != nil {
		if err := r.RequestHeaderModifier.Validate(); err != nil {
			return err
		}
		if r.ModelRouting != nil {
			for _, name := range aiManagedRequestHeaders {
				if r.RequestHeaderModifier.contains(name) {
					return errors.New("模型路由的请求 Header 改写不能使用系统管理的名称")
				}
			}
		}
	}
	if r.ResponseHeaderModifier != nil {
		if err := r.ResponseHeaderModifier.Validate(); err != nil {
			return err
		}
	}
	if r.Timeout != nil {
		if r.Timeout.RequestMillis < minRouteTimeoutMillis || r.Timeout.RequestMillis > maxRouteTimeoutMillis {
			return errors.New("请求超时必须在 100-300000 毫秒之间")
		}
		totalTimeoutMillis = r.Timeout.RequestMillis
	}
	if r.Retry != nil {
		if r.ModelRouting != nil {
			return errors.New("模型路由暂不支持失败重试")
		}
		if r.Retry.Attempts < minRetryAttempts || r.Retry.Attempts > maxRetryAttempts {
			return errors.New("重试次数必须在 1-5 之间")
		}
		if r.Retry.PerTryTimeoutMillis < minPerTryTimeoutMillis || r.Retry.PerTryTimeoutMillis > maxPerTryTimeoutMillis {
			return errors.New("单次重试超时必须在 100-60000 毫秒之间")
		}
		if r.Retry.PerTryTimeoutMillis > totalTimeoutMillis {
			return errors.New("单次重试超时不能大于请求总超时")
		}
	}

	return nil
}

// Validate 校验并规整 Header 改写配置
func (r *HeaderModifierReq) Validate() error {
	setHeaders := make([]HeaderValueReq, 0, len(r.Set))
	for _, header := range r.Set {
		name := strings.TrimSpace(strings.ToLower(header.Name))
		value := strings.TrimSpace(header.Value)
		if name == "" {
			return errors.New("Header 名称不能为空")
		}
		if value == "" {
			return errors.New("Header 值不能为空")
		}
		setHeaders = append(setHeaders, HeaderValueReq{Name: name, Value: value})
	}

	removeHeaders := lowerNonEmptyStrings(r.Remove)
	if len(setHeaders) == 0 && len(removeHeaders) == 0 {
		return errors.New("至少需要配置一个 Header 写入或删除动作")
	}
	r.Set = setHeaders
	r.Remove = removeHeaders
	return nil
}

func (r *HeaderModifierReq) contains(name string) bool {
	for _, header := range r.Set {
		if strings.EqualFold(header.Name, name) {
			return true
		}
	}
	for _, header := range r.Remove {
		if strings.EqualFold(header, name) {
			return true
		}
	}
	return false
}

func lowerNonEmptyStrings(items []string) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(strings.ToLower(item))
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func validHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func validHostname(hostname string) bool {
	if len(hostname) > 2 && hostname[:2] == "*." {
		hostname = hostname[2:]
	}
	hostname = strings.ToLower(hostname)
	if hostname == "" || len(hostname) > 253 {
		return false
	}
	for label := range strings.SplitSeq(hostname, ".") {
		if !validDNSLabel(label) {
			return false
		}
	}
	return true
}

func validDNSLabel(label string) bool {
	if label == "" || len(label) > 63 {
		return false
	}
	for i, r := range label {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-'
		if !valid {
			return false
		}
		if (i == 0 || i == len(label)-1) && r == '-' {
			return false
		}
	}
	return true
}

// Validate 校验控制台提交的 Route 启停请求体
func (r SetRouteEnabledReq) Validate() error {
	if r.Enabled == nil {
		return errors.New("启用状态不能为空")
	}
	return nil
}

// Value 返回已校验的启停值
func (r SetRouteEnabledReq) Value() bool {
	return *r.Enabled
}
