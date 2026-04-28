// Package resource 定义声明式控制面资源
package resource

// Kind 表示声明式资源类型
type Kind string

const (
	// KindGateway 表示 Gateway 资源类型
	KindGateway Kind = "Gateway"
	// KindRoute 表示 Route 资源类型
	KindRoute Kind = "Route"
	// KindUpstream 表示 Upstream 资源类型
	KindUpstream Kind = "Upstream"
	// KindAuthPolicy 表示 AuthPolicy 资源类型
	KindAuthPolicy Kind = "AuthPolicy"
)

// AuthType 表示认证策略类型
type AuthType string

const (
	// AuthTypeAPIKey 表示 API Key 认证
	AuthTypeAPIKey AuthType = "APIKey"
)

// Bundle 表示一次内存编译所需的资源集合
type Bundle struct {
	Gateways       []Gateway       `json:"gateways"`
	Routes         []Route         `json:"routes"`
	Upstreams      []Upstream      `json:"upstreams"`
	AuthPolicies   []AuthPolicy    `json:"authPolicies"`
	PolicyBindings []PolicyBinding `json:"policyBindings"`
}

// Metadata 标识一个声明式资源
type Metadata struct {
	Name string `json:"name"`
}

// Gateway 声明一个流量入口
type Gateway struct {
	Metadata Metadata    `json:"metadata"`
	Spec     GatewaySpec `json:"spec"`
}

// GatewaySpec 定义 Gateway 的监听器
type GatewaySpec struct {
	Listeners []Listener `json:"listeners"`
}

// Listener 声明一个 Gateway 监听端口
type Listener struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Hostname string `json:"hostname"`
}

// Route 声明请求匹配规则和 Upstream 引用
type Route struct {
	Metadata Metadata  `json:"metadata"`
	Spec     RouteSpec `json:"spec"`
}

// RouteSpec 定义 Route 如何挂载到 Gateway
type RouteSpec struct {
	ParentRefs []string    `json:"parentRefs"`
	Hostnames  []string    `json:"hostnames"`
	Rules      []RouteRule `json:"rules"`
}

// RouteRule 声明一条路由匹配规则和加权 Upstream 集合
type RouteRule struct {
	PathPrefix    string        `json:"pathPrefix"`
	Methods       []string      `json:"methods"`
	TimeoutMillis int           `json:"timeoutMillis"`
	Headers       []HeaderMatch `json:"headers"`
	UpstreamRefs  []UpstreamRef `json:"upstreamRefs"`
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

// Upstream 声明一个逻辑上游服务
type Upstream struct {
	Metadata Metadata     `json:"metadata"`
	Spec     UpstreamSpec `json:"spec"`
}

// UpstreamSpec 定义 Upstream 的端点集合
type UpstreamSpec struct {
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint 声明一个上游端点
type Endpoint struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// AuthPolicy 声明认证策略
type AuthPolicy struct {
	Metadata Metadata       `json:"metadata"`
	Spec     AuthPolicySpec `json:"spec"`
}

// AuthPolicySpec 定义认证策略配置
type AuthPolicySpec struct {
	Type   AuthType   `json:"type"`
	APIKey APIKeyAuth `json:"apiKey"`
}

// APIKeyAuth 定义 API Key 提取位置
type APIKeyAuth struct {
	Header string `json:"header"`
	Query  string `json:"query"`
}

// PolicyBinding 声明一组策略绑定到哪个资源
type PolicyBinding struct {
	Metadata Metadata          `json:"metadata"`
	Spec     PolicyBindingSpec `json:"spec"`
}

// PolicyBindingSpec 定义策略绑定目标和策略引用
type PolicyBindingSpec struct {
	TargetRef PolicyTargetRef `json:"targetRef"`
	Policies  []PolicyRef     `json:"policies"`
}

// PolicyTargetRef 表示策略绑定目标资源
type PolicyTargetRef struct {
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}

// PolicyRef 表示被绑定的策略资源引用
type PolicyRef struct {
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}
