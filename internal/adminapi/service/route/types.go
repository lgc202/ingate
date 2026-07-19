package route

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 是 Route 列表查询结果
type ListResult struct {
	Routes []resource.Route
}

// RouteResult 是单个路由用例结果
type RouteResult struct {
	Route *resource.Route
}

// CreateRouteParams 是创建 Route 用例参数
type CreateRouteParams struct {
	Name       string
	GatewayIDs []string
	Hostnames  []string
	Enabled    bool
	Rules      []RouteRuleParams
}

// UpdateRouteParams 是更新 Route 用例参数
type UpdateRouteParams struct {
	Version string
	CreateRouteParams
}

// RouteRuleParams 是 Route 单条规则用例参数
type RouteRuleParams struct {
	Name                   string
	PathPrefix             string
	Methods                []string
	Headers                []HeaderMatchParams
	Targets                []TargetParams
	ModelRouting           *ModelRoutingParams
	RequestHeaderModifier  *HeaderModifierParams
	ResponseHeaderModifier *HeaderModifierParams
	Timeout                *RouteTimeoutParams
	Retry                  *RouteRetryParams
}

// ModelRoutingParams 是模型路由用例参数
type ModelRoutingParams struct {
	Models []ModelRouteParams
}

// ModelRouteParams 是单个客户端模型到模型服务和厂商模型的映射参数
type ModelRouteParams struct {
	Model         string
	UpstreamID    string
	UpstreamModel string
}

// HeaderMatchParams 是 Route header 匹配条件参数
type HeaderMatchParams struct {
	Name  string
	Value string
}

// TargetParams 是 Route 目标 Upstream 用例参数
type TargetParams struct {
	UpstreamID string
	Weight     int
}

// HeaderModifierParams 是 Route 原生 Header 改写参数
type HeaderModifierParams struct {
	Set    []HeaderValueParams
	Remove []string
}

// HeaderValueParams 是 Header 写入参数
type HeaderValueParams struct {
	Name  string
	Value string
}

// RouteTimeoutParams 是 Route 原生超时参数
type RouteTimeoutParams struct {
	RequestMillis int
}

// RouteRetryParams 是 Route 原生重试参数
type RouteRetryParams struct {
	Attempts            int
	PerTryTimeoutMillis int
}
