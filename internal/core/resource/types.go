package resource

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

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

// Bundle 表示一次编译所需的资源集合
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

// ResourceStatus 表示声明式资源状态
type ResourceStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Gateway 声明一个流量入口
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec    `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// GatewayList 表示 Gateway 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Gateway `json:"items"`
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
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
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
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AIRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIRouteSpec    `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// AIRouteList 表示 AIRoute 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AIRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AIRoute `json:"items"`
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
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Upstream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpstreamSpec   `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// UpstreamList 表示 Upstream 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UpstreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Upstream `json:"items"`
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
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AIProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIProviderSpec `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// AIProviderList 表示 AIProvider 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AIProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AIProvider `json:"items"`
}

// AIProviderSpec 定义 AI 模型供应商入口
type AIProviderSpec struct {
	Type     AIProviderType `json:"type"`
	Endpoint string         `json:"endpoint"`
	Models   []string       `json:"models"`
}

// Plugin 声明一个可绑定到网关资源的插件
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Plugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PluginSpec     `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// PluginList 表示 Plugin 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Plugin `json:"items"`
}

// PluginSpec 定义插件运行时和入口
type PluginSpec struct {
	Runtime  PluginRuntime `json:"runtime"`
	Version  string        `json:"version"`
	Endpoint string        `json:"endpoint"`
	Image    string        `json:"image"`
}

// AuthPolicy 声明认证策略
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AuthPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthPolicySpec `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// AuthPolicyList 表示 AuthPolicy 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AuthPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AuthPolicy `json:"items"`
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
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RateLimitPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RateLimitPolicySpec `json:"spec,omitempty"`
	Status ResourceStatus      `json:"status,omitempty"`
}

// RateLimitPolicyList 表示 RateLimitPolicy 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RateLimitPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RateLimitPolicy `json:"items"`
}

// RateLimitPolicySpec 定义限流策略配置
type RateLimitPolicySpec struct {
	Requests      int          `json:"requests"`
	WindowSeconds int          `json:"windowSeconds"`
	KeyBy         RateLimitKey `json:"keyBy"`
	Header        string       `json:"header"`
}

// PluginBinding 声明一组插件绑定到哪个资源
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PluginBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PluginBindingSpec `json:"spec,omitempty"`
	Status ResourceStatus    `json:"status,omitempty"`
}

// PluginBindingList 表示 PluginBinding 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PluginBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PluginBinding `json:"items"`
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
	Name   string               `json:"name"`
	Config runtime.RawExtension `json:"config,omitempty"`
}

// PolicyBinding 声明一组策略绑定到哪个资源
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PolicyBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicyBindingSpec `json:"spec,omitempty"`
	Status ResourceStatus    `json:"status,omitempty"`
}

// PolicyBindingList 表示 PolicyBinding 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PolicyBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PolicyBinding `json:"items"`
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
