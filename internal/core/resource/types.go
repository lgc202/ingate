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
	// KindRateLimitPolicy 表示 RateLimitPolicy 资源类型
	KindRateLimitPolicy Kind = "RateLimitPolicy"
	// KindPlugin 表示 Plugin 资源类型
	KindPlugin Kind = "Plugin"
	// KindAIProvider 表示 AIProvider 资源类型
	KindAIProvider Kind = "AIProvider"
	// KindAIRoute 表示 AIRoute 资源类型
	KindAIRoute Kind = "AIRoute"
)

// AIProviderType 表示 AI 供应商协议类型
type AIProviderType string

const (
	// AIProviderTypeOpenAICompatible 表示 OpenAI 兼容协议
	AIProviderTypeOpenAICompatible AIProviderType = "OpenAICompatible"
)

// PluginRuntime 表示插件运行时类型
type PluginRuntime string

const (
	// PluginRuntimeExternal 表示外部进程插件
	PluginRuntimeExternal PluginRuntime = "External"
	// PluginRuntimeWASM 表示 WASM 插件
	PluginRuntimeWASM PluginRuntime = "WASM"
)

// AuthType 表示认证策略类型
type AuthType string

const (
	// AuthTypeAPIKey 表示 API Key 认证
	AuthTypeAPIKey AuthType = "APIKey"
)

// RateLimitKey 表示限流计数维度
type RateLimitKey string

const (
	// RateLimitKeyIP 表示按客户端 IP 限流
	RateLimitKeyIP RateLimitKey = "IP"
	// RateLimitKeyHeader 表示按请求 header 限流
	RateLimitKeyHeader RateLimitKey = "Header"
)

// Bundle 表示一次内存编译所需的资源集合
type Bundle struct {
	Gateways          []Gateway         `json:"gateways"`
	Routes            []Route           `json:"routes"`
	AIRoutes          []AIRoute         `json:"aiRoutes"`
	Upstreams         []Upstream        `json:"upstreams"`
	AIProviders       []AIProvider      `json:"aiProviders"`
	Plugins           []Plugin          `json:"plugins"`
	AuthPolicies      []AuthPolicy      `json:"authPolicies"`
	RateLimitPolicies []RateLimitPolicy `json:"rateLimitPolicies"`
	PolicyBindings    []PolicyBinding   `json:"policyBindings"`
	PluginBindings    []PluginBinding   `json:"pluginBindings"`
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

// AIRoute 声明 AI 请求匹配规则和模型供应商引用
type AIRoute struct {
	Metadata Metadata    `json:"metadata"`
	Spec     AIRouteSpec `json:"spec"`
}

// AIRouteSpec 定义 AI 路由如何挂载到 Gateway
type AIRouteSpec struct {
	ParentRefs   []string        `json:"parentRefs"`
	PathPrefix   string          `json:"pathPrefix"`
	Model        string          `json:"model"`
	ProviderRefs []AIProviderRef `json:"providerRefs"`
}

// AIProviderRef 表示 AIRoute 中的 AIProvider 引用
type AIProviderRef struct {
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

// AIProvider 声明一个 AI 模型供应商
type AIProvider struct {
	Metadata Metadata       `json:"metadata"`
	Spec     AIProviderSpec `json:"spec"`
}

// AIProviderSpec 定义 AI 模型供应商入口
type AIProviderSpec struct {
	Type     AIProviderType `json:"type"`
	Endpoint string         `json:"endpoint"`
	Models   []string       `json:"models"`
}

// Plugin 声明一个可绑定到网关资源的插件
type Plugin struct {
	Metadata Metadata   `json:"metadata"`
	Spec     PluginSpec `json:"spec"`
}

// PluginSpec 定义插件运行时和入口
type PluginSpec struct {
	Runtime  PluginRuntime `json:"runtime"`
	Version  string        `json:"version"`
	Endpoint string        `json:"endpoint"`
	Image    string        `json:"image"`
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

// RateLimitPolicy 声明限流策略
type RateLimitPolicy struct {
	Metadata Metadata            `json:"metadata"`
	Spec     RateLimitPolicySpec `json:"spec"`
}

// RateLimitPolicySpec 定义限流策略配置
type RateLimitPolicySpec struct {
	Requests      int          `json:"requests"`
	WindowSeconds int          `json:"windowSeconds"`
	KeyBy         RateLimitKey `json:"keyBy"`
	Header        string       `json:"header"`
}

// PluginBinding 声明一组插件绑定到哪个资源
type PluginBinding struct {
	Metadata Metadata          `json:"metadata"`
	Spec     PluginBindingSpec `json:"spec"`
}

// PluginBindingSpec 定义插件绑定目标和插件引用
type PluginBindingSpec struct {
	TargetRef PluginTargetRef `json:"targetRef"`
	Plugins   []PluginRef     `json:"plugins"`
}

// PluginTargetRef 表示插件绑定目标资源
type PluginTargetRef struct {
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}

// PluginRef 表示被绑定的插件引用
type PluginRef struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
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
