package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RouteFilterType 表示 Route 原生请求处理能力类型
type RouteFilterType string

const (
	// RouteFilterRequestHeaderModifier 表示修改上游请求 header
	RouteFilterRequestHeaderModifier RouteFilterType = "RequestHeaderModifier"
	// RouteFilterResponseHeaderModifier 表示修改下游响应 header
	RouteFilterResponseHeaderModifier RouteFilterType = "ResponseHeaderModifier"
	// RouteFilterURLRewrite 表示改写请求 URL
	RouteFilterURLRewrite RouteFilterType = "URLRewrite"
	// RouteFilterRequestMirror 表示镜像请求到旁路上游
	RouteFilterRequestMirror RouteFilterType = "RequestMirror"
	// RouteFilterCORS 表示当前 Route 的 CORS 响应策略
	RouteFilterCORS RouteFilterType = "CORS"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Route 声明请求匹配规则和 Upstream 引用
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

// RouteSpec 定义 Route 如何挂载到 Gateway
type RouteSpec struct {
	// DisplayName 保存控制台展示名称，不参与引用和运行时匹配
	DisplayName string `json:"displayName,omitempty"`
	// Enabled 表示 Route 是否参与编译和下发
	Enabled bool `json:"enabled"`
	// +listType=atomic
	ParentRefs []ParentRef `json:"parentRefs"`
	// +listType=atomic
	Hostnames []string `json:"hostnames"`
	// +listType=atomic
	Rules []RouteRule `json:"rules"`
}

// ParentRef 引用承载当前 Route 的 Gateway
type ParentRef struct {
	Name string `json:"name"`
}

// RouteRule 声明一条路由匹配规则和加权 Upstream 集合
type RouteRule struct {
	Name       string `json:"name"`
	PathPrefix string `json:"pathPrefix"`
	// +listType=atomic
	Methods []string `json:"methods"`
	// +listType=atomic
	Headers []HeaderMatch `json:"headers"`
	// +listType=atomic
	Filters []RouteFilter `json:"filters,omitempty"`
	Timeout *RouteTimeout `json:"timeout,omitempty"`
	Retry   *RouteRetry   `json:"retry,omitempty"`
	// +listType=atomic
	UpstreamRefs []UpstreamRef `json:"upstreamRefs"`
}

// RouteFilter 声明命中 RouteRule 后执行的原生请求处理能力
type RouteFilter struct {
	Type                   RouteFilterType `json:"type"`
	RequestHeaderModifier  *HeaderModifier `json:"requestHeaderModifier,omitempty"`
	ResponseHeaderModifier *HeaderModifier `json:"responseHeaderModifier,omitempty"`
}

// HeaderModifier 表示 header 写入和删除动作
type HeaderModifier struct {
	// +listType=atomic
	Set []HeaderValue `json:"set,omitempty"`
	// +listType=atomic
	Add []HeaderValue `json:"add,omitempty"`
	// +listType=atomic
	Remove []string `json:"remove,omitempty"`
}

// HeaderValue 表示 header 名和值
type HeaderValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RouteTimeout 表示当前 RouteRule 的请求总超时
type RouteTimeout struct {
	RequestMillis int `json:"requestMillis,omitempty"`
}

// RouteRetry 表示当前 RouteRule 的失败重试策略
type RouteRetry struct {
	Attempts            int `json:"attempts,omitempty"`
	PerTryTimeoutMillis int `json:"perTryTimeoutMillis,omitempty"`
	// +listType=atomic
	RetryOn []string `json:"retryOn,omitempty"`
}

// HeaderMatch 表示 HTTP header 精确匹配条件
type HeaderMatch struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// UpstreamRef 表示 RouteRule 中的 Upstream 引用
type UpstreamRef struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}
