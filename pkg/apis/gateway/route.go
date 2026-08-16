package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// PathMatchType 表示 Route 路径匹配方式
type PathMatchType string

const (
	// PathMatchPrefix 表示按路径前缀匹配
	PathMatchPrefix PathMatchType = "Prefix"
	// PathMatchExact 表示按完整路径匹配
	PathMatchExact PathMatchType = "Exact"
)

// HostRewriteMode 表示转发请求时如何生成上游 Host
type HostRewriteMode string

const (
	// HostRewriteServiceAddress 使用实际选中的服务端点主机名
	HostRewriteServiceAddress HostRewriteMode = "ServiceAddress"
	// HostRewritePreserve 保留客户端请求中的 Host
	HostRewritePreserve HostRewriteMode = "Preserve"
	// HostRewriteCustom 使用用户指定的固定主机名
	HostRewriteCustom HostRewriteMode = "Custom"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
	// UpstreamRefs 保存接收请求的上游服务及流量权重
	// +listType=atomic
	UpstreamRefs []UpstreamRef `json:"upstreamRefs,omitempty"`
	// HostRewrite 为空时保留客户端 Host，其他模式由 Route 显式控制
	HostRewrite            *HostRewrite    `json:"hostRewrite,omitempty"`
	RequestHeaderModifier  *HeaderModifier `json:"requestHeaderModifier,omitempty"`
	ResponseHeaderModifier *HeaderModifier `json:"responseHeaderModifier,omitempty"`
	Timeout                *RouteTimeout   `json:"timeout,omitempty"`
	Retry                  *RouteRetry     `json:"retry,omitempty"`
}

// HostRewrite 定义转发请求使用的上游 Host
type HostRewrite struct {
	Mode HostRewriteMode `json:"mode"`
	// Hostname 仅在 Custom 模式下使用，不包含端口
	Hostname string `json:"hostname,omitempty"`
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
