// Package route 定义 Route 管理接口的请求和响应模型
package route

import admindto "github.com/lgc202/ingate/internal/adminapi/dto"

// ListRoutesResp 是 Route 列表响应
type ListRoutesResp struct {
	Routes []Route `json:"routes"`
}

// Route 是 admin-api 面向控制台返回的路由对象，不直接暴露 CR 结构
type Route struct {
	ID         string                  `json:"id"`
	Version    string                  `json:"version,omitempty"`
	Status     admindto.ResourceStatus `json:"status"`
	Name       string                  `json:"name"`
	GatewayIDs []string                `json:"gatewayIDs"`
	Hostnames  []string                `json:"hostnames"`
	Rules      []RouteRule             `json:"rules"`
	Enabled    bool                    `json:"enabled"`
	CreatedAt  string                  `json:"createdAt"`
}

// CreateRouteReq 是创建 Route 的请求体
type CreateRouteReq struct {
	Name       string      `json:"name"`
	GatewayIDs []string    `json:"gatewayIDs"`
	Hostnames  []string    `json:"hostnames"`
	Enabled    *bool       `json:"enabled,omitempty"`
	Rules      []RouteRule `json:"rules"`
}

// UpdateRouteReq 是更新 Route 的请求体
type UpdateRouteReq struct {
	Version string `json:"version"`
	CreateRouteReq
}

// RouteRule 是控制台读写的一条 Route 规则
type RouteRule struct {
	Name                   string             `json:"name"`
	PathPrefix             string             `json:"pathPrefix"`
	Methods                []string           `json:"methods,omitempty"`
	Headers                []HeaderMatchReq   `json:"headers,omitempty"`
	Targets                []RouteTarget      `json:"targets,omitempty"`
	ModelRouting           *ModelRouting      `json:"modelRouting,omitempty"`
	RequestHeaderModifier  *HeaderModifierReq `json:"requestHeaderModifier,omitempty"`
	ResponseHeaderModifier *HeaderModifierReq `json:"responseHeaderModifier,omitempty"`
	Timeout                *RouteTimeoutReq   `json:"timeout,omitempty"`
	Retry                  *RouteRetryReq     `json:"retry,omitempty"`
}

// ModelRouting 是控制台读写的模型路由配置
type ModelRouting struct {
	UpstreamID string       `json:"upstreamID"`
	Models     []ModelRoute `json:"models"`
}

// ModelRoute 将客户端模型名称映射到上游模型名称
type ModelRoute struct {
	Model         string `json:"model"`
	UpstreamModel string `json:"upstreamModel,omitempty"`
}

// HeaderMatchReq 是控制台提交的 Header 精确匹配条件
type HeaderMatchReq struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RouteTarget 是路由转发到的 Upstream 及其权重
type RouteTarget struct {
	UpstreamID string `json:"upstreamID"`
	Weight     int    `json:"weight"`
}

// HeaderModifierReq 是控制台提交的 Header 改写配置
type HeaderModifierReq struct {
	Set    []HeaderValueReq `json:"set,omitempty"`
	Remove []string         `json:"remove,omitempty"`
}

// HeaderValueReq 是控制台提交的 Header 写入配置
type HeaderValueReq struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RouteTimeoutReq 是控制台提交的路由总超时配置
type RouteTimeoutReq struct {
	RequestMillis int `json:"requestMillis"`
}

// RouteRetryReq 是控制台提交的失败重试配置
type RouteRetryReq struct {
	Attempts            int `json:"attempts"`
	PerTryTimeoutMillis int `json:"perTryTimeoutMillis"`
}

// SetRouteEnabledReq 是控制台启停 Route 的请求体
type SetRouteEnabledReq struct {
	Enabled *bool `json:"enabled"`
}

// CreateRouteResp 是创建 Route 的响应
type CreateRouteResp struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
}

// UpdateRouteResp 是更新 Route 的响应
type UpdateRouteResp struct {
	Success bool `json:"success"`
}

// SetRouteEnabledResp 是启停 Route 的响应
type SetRouteEnabledResp struct {
	Success bool `json:"success"`
}

// DeleteRouteResp 是删除 Route 的响应
type DeleteRouteResp struct {
	Success bool `json:"success"`
}
