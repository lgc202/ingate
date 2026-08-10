package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// PathMatchType 表示 Route 路径匹配方式
type PathMatchType string

const (
	// PathMatchPrefix 表示按路径前缀匹配
	PathMatchPrefix PathMatchType = "Prefix"
	// PathMatchExact 表示按完整路径匹配
	PathMatchExact PathMatchType = "Exact"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// Route 声明一组请求匹配条件和对应的转发行为
type Route struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteSpec      `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// RouteList 表示 Route 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Route `json:"items"`
}

// RouteSpec 定义 Route 的挂载范围、匹配条件和转发行为
type RouteSpec struct {
	// DisplayName 保存控制台展示名称，不参与引用和请求匹配
	DisplayName string `json:"displayName,omitempty"`
	// Enabled 表示 Route 是否参与编译和下发
	Enabled bool `json:"enabled"`
	// GatewayRefs 保存承载当前 Route 的 Gateway ID
	// +listType=atomic
	GatewayRefs []string `json:"gatewayRefs"`
	// Hostnames 为空时不限制请求 Host，多个值之间是 OR 关系
	// +listType=atomic
	Hostnames []string   `json:"hostnames,omitempty"`
	Match     RouteMatch `json:"match"`
	// UpstreamRefs 和 ModelRouting 必须且只能配置一个
	// +listType=atomic
	UpstreamRefs           []UpstreamRef   `json:"upstreamRefs,omitempty"`
	ModelRouting           *ModelRouting   `json:"modelRouting,omitempty"`
	RequestHeaderModifier  *HeaderModifier `json:"requestHeaderModifier,omitempty"`
	ResponseHeaderModifier *HeaderModifier `json:"responseHeaderModifier,omitempty"`
	Timeout                *RouteTimeout   `json:"timeout,omitempty"`
	Retry                  *RouteRetry     `json:"retry,omitempty"`
}

// RouteMatch 表示必须同时满足的请求匹配条件
type RouteMatch struct {
	Path PathMatch `json:"path"`
	// Methods 为空时匹配所有 HTTP 方法，多个值之间是 OR 关系
	// +listType=atomic
	Methods []string `json:"methods,omitempty"`
	// Headers 必须全部匹配
	// +listType=atomic
	Headers []HeaderMatch `json:"headers,omitempty"`
}

// PathMatch 表示请求路径匹配条件
type PathMatch struct {
	Type  PathMatchType `json:"type"`
	Value string        `json:"value"`
}

// ModelRouting 声明公开模型名称与实际模型服务之间的映射
type ModelRouting struct {
	// +listType=atomic
	Models []ModelMapping `json:"models"`
}

// ModelMapping 将客户端模型名称映射到一个模型 Upstream 及其厂商模型名称
type ModelMapping struct {
	// Model 是客户端请求中使用的公开模型名称
	Model string `json:"model"`
	// UpstreamRef 引用实际接收该模型请求的 Upstream
	UpstreamRef string `json:"upstreamRef"`
	// UpstreamModel 是发送给模型服务的模型名称，为空时沿用 Model
	UpstreamModel string `json:"upstreamModel,omitempty"`
}

// HeaderModifier 表示 Header 写入和删除动作
type HeaderModifier struct {
	// +listType=atomic
	Set []HeaderValue `json:"set,omitempty"`
	// +listType=atomic
	Add []HeaderValue `json:"add,omitempty"`
	// +listType=atomic
	Remove []string `json:"remove,omitempty"`
}

// HeaderValue 表示 Header 名和值
type HeaderValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RouteTimeout 表示 Route 的请求总超时
type RouteTimeout struct {
	RequestMillis int `json:"requestMillis"`
}

// RouteRetry 表示 Route 的失败重试配置
type RouteRetry struct {
	Attempts            int `json:"attempts"`
	PerTryTimeoutMillis int `json:"perTryTimeoutMillis"`
}

// HeaderMatch 表示 HTTP Header 精确匹配条件
type HeaderMatch struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// UpstreamRef 表示 Route 转发到的 Upstream 及其相对权重
type UpstreamRef struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}
