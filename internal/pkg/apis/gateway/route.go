package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RouteAccessMode 表示 Route 是否要求调用方身份。
type RouteAccessMode string

const (
	// RouteAccessPublic 表示请求不需要携带访问密钥。
	RouteAccessPublic RouteAccessMode = "Public"
	// RouteAccessCaller 表示请求必须使用已授权 Caller 的访问密钥。
	RouteAccessCaller RouteAccessMode = "Caller"
)

// PathMatchType 表示 Route 路径匹配方式。
type PathMatchType string

const (
	// PathMatchPrefix 表示按路径前缀匹配。
	PathMatchPrefix PathMatchType = "Prefix"
	// PathMatchExact 表示按完整路径匹配。
	PathMatchExact PathMatchType = "Exact"
)

// HostRewriteMode 表示转发请求时如何生成上游 Host。
type HostRewriteMode string

const (
	// HostRewriteUpstreamHost 使用实际选中的 Upstream 端点主机名。
	HostRewriteUpstreamHost HostRewriteMode = "UpstreamHost"
	// HostRewritePreserve 保留客户端请求中的 Host。
	HostRewritePreserve HostRewriteMode = "Preserve"
	// HostRewriteCustom 使用用户指定的固定主机名。
	HostRewriteCustom HostRewriteMode = "Custom"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Route 声明一组请求匹配条件和对应的转发行为。
type Route struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteSpec      `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// RouteList 表示 Route 资源列表。
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Route `json:"items"`
}

// RouteSpec 定义 Route 的挂载范围、匹配条件和转发行为。
type RouteSpec struct {
	// DisplayName 保存控制台展示名称，不参与引用和请求匹配。
	DisplayName string `json:"displayName,omitempty"`
	// Enabled 表示 Route 是否参与编译和下发。
	Enabled bool `json:"enabled"`
	// AccessMode 决定客户端是否需要使用 Caller 访问密钥。
	AccessMode RouteAccessMode `json:"accessMode"`
	// GatewayRefs 保存承载当前 Route 的 Gateway ID 列表。
	// +listType=atomic
	GatewayRefs []string `json:"gatewayRefs"`
	// Hostnames 为空时继承所关联 Gateway Listener 的 Host 范围，多个值之间是 OR 关系。
	// +listType=atomic
	Hostnames []string `json:"hostnames,omitempty"`
	// Match 定义请求必须满足的路径、方法和 Header 条件。
	Match RouteMatch `json:"match"`
	// UpstreamRefs 保存接收请求的上游服务及流量权重。
	// AI 为空时使用该字段；AI Route 的线路由 AI.Models 分别声明。
	// +listType=atomic
	UpstreamRefs []UpstreamRef `json:"upstreamRefs,omitempty"`
	// AI 存在时表示当前 Route 发布 OpenAI 兼容模型接口。
	AI *AIRoute `json:"ai,omitempty"`
	// HostRewrite 由 API Server 物化默认值，存储后的配置始终显式声明 Host 行为。
	HostRewrite HostRewrite `json:"hostRewrite"`
	// RequestHeaderModifier 在请求转发到 Upstream 前修改 Header。
	RequestHeaderModifier *HeaderModifier `json:"requestHeaderModifier,omitempty"`
	// ResponseHeaderModifier 在 Upstream 响应返回客户端前修改 Header。
	ResponseHeaderModifier *HeaderModifier `json:"responseHeaderModifier,omitempty"`
	// Timeout 由 API Server 物化默认值，Compiler 不依赖数据面的隐式超时。
	Timeout RouteTimeout `json:"timeout"`
	// Retry 为空时不重试失败的 Upstream 请求。
	Retry *RouteRetry `json:"retry,omitempty"`
}

// RouteMatch 表示必须同时满足的请求匹配条件。
type RouteMatch struct {
	// Path 是每个请求都必须满足的路径条件。
	Path PathMatch `json:"path"`
	// Methods 为空时匹配所有 HTTP 方法，多个值之间是 OR 关系。
	// +listType=atomic
	Methods []string `json:"methods,omitempty"`
	// Headers 必须全部匹配。
	// +listType=atomic
	Headers []HeaderMatch `json:"headers,omitempty"`
}

// PathMatch 表示请求路径匹配条件。
type PathMatch struct {
	// Type 选择完整路径或路径前缀匹配。
	Type PathMatchType `json:"type"`
	// Value 是不含查询参数和片段的绝对路径。
	Value string `json:"value"`
}

// HeaderMatch 表示 HTTP Header 精确匹配条件。
type HeaderMatch struct {
	// Name 不区分大小写。
	Name string `json:"name"`
	// Value 按完整字符串匹配。
	Value string `json:"value"`
}

// UpstreamRef 表示 Route 转发到的 Upstream 及其相对权重。
type UpstreamRef struct {
	// Name 引用 Upstream 的 metadata.name。
	Name string `json:"name"`
	// Weight 是多个 Upstream 之间的相对流量权重。
	Weight int `json:"weight"`
}

// AIRoute 定义一个入口路径下发布的客户端模型及其实际模型线路。
type AIRoute struct {
	// Models 中每个客户端模型名在当前 Route 内必须唯一。
	// +listType=atomic
	Models []AIModel `json:"models"`
}

// AIModel 把调用方使用的稳定模型名映射到一个或多个模型服务。
type AIModel struct {
	// Name 是调用方请求体中使用的稳定模型名。
	Name string `json:"name"`
	// Targets 是可承载该模型的实际线路，各线路按相对权重分流。
	// +listType=atomic
	Targets []AIModelTarget `json:"targets"`
}

// AIModelTarget 表示一个模型服务上的实际模型线路。
type AIModelTarget struct {
	// UpstreamRef 引用模型服务的资源 ID。
	UpstreamRef string `json:"upstreamRef"`
	// Model 是发送给模型服务的真实模型名。
	Model string `json:"model"`
	// Weight 是同一客户端模型下各线路的相对权重。
	Weight int `json:"weight"`
}

// HostRewrite 定义转发请求使用的上游 Host。
type HostRewrite struct {
	// Mode 选择 Upstream 端点主机名、原始 Host 或固定主机名。
	Mode HostRewriteMode `json:"mode"`
	// Hostname 仅在 Custom 模式下使用，不包含端口。
	Hostname string `json:"hostname,omitempty"`
}

// HeaderModifier 表示 Header 写入和删除动作。
type HeaderModifier struct {
	// Set 覆盖同名 Header 的现有值，不存在时创建。
	// +listType=atomic
	Set []HeaderValue `json:"set,omitempty"`
	// Add 在同名 Header 的现有值之后追加值，不存在时创建。
	// +listType=atomic
	Add []HeaderValue `json:"add,omitempty"`
	// Remove 删除指定 Header。
	// +listType=atomic
	Remove []string `json:"remove,omitempty"`
}

// HeaderValue 表示 Header 名和值。
type HeaderValue struct {
	// Name 是待写入的 Header 名称。
	Name string `json:"name"`
	// Value 是待写入的非空 Header 值。
	Value string `json:"value"`
}

// RouteTimeout 表示 Route 的请求总超时。
type RouteTimeout struct {
	// RequestMillis 是从接收请求到返回响应的总超时毫秒数。
	RequestMillis int `json:"requestMillis"`
}

// RouteRetry 表示 Route 的失败重试配置。
type RouteRetry struct {
	// Attempts 是首次转发失败后的最大重试次数。
	Attempts int `json:"attempts"`
	// PerTryTimeoutMillis 是每次转发尝试（包括首次）的超时毫秒数。
	PerTryTimeoutMillis int `json:"perTryTimeoutMillis"`
}
