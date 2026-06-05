package gateway

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Kind 表示声明式资源类型
type Kind string

const (
	// KindGateway 表示 Gateway 资源类型
	KindGateway Kind = "Gateway"
	// KindRuntimeGroup 表示 RuntimeGroup 资源类型
	KindRuntimeGroup Kind = "RuntimeGroup"
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
	// KindPluginBinding 表示 PluginBinding 资源类型
	KindPluginBinding Kind = "PluginBinding"
	// KindAIProvider 表示 AIProvider 资源类型
	KindAIProvider Kind = "AIProvider"
	// KindAIModel 表示 AIModel 资源类型
	KindAIModel Kind = "AIModel"
	// KindAIRoute 表示 AIRoute 资源类型
	KindAIRoute Kind = "AIRoute"
	// KindAIPolicy 表示 AIPolicy 资源类型
	KindAIPolicy Kind = "AIPolicy"
)

// AIProviderType 表示 AI 供应商协议类型
type AIProviderType string

const (
	// AIProviderTypeOpenAICompatible 表示 OpenAI 兼容协议
	AIProviderTypeOpenAICompatible AIProviderType = "OpenAICompatible"
)

// AIExecutionTargetType 表示 AI 请求执行目标类型
type AIExecutionTargetType string

const (
	// AIExecutionTargetTypeWasm 表示 Envoy Wasm 插件执行目标
	AIExecutionTargetTypeWasm AIExecutionTargetType = "Wasm"
	// AIExecutionTargetTypeExternalProcessor 表示 Envoy External Processor 执行目标
	AIExecutionTargetTypeExternalProcessor AIExecutionTargetType = "ExternalProcessor"
	// AIExecutionTargetTypeGoRuntime 表示 Go AI Runtime 执行目标
	AIExecutionTargetTypeGoRuntime AIExecutionTargetType = "GoRuntime"
)

// PluginRuntime 表示插件运行时类型
type PluginRuntime string

const (
	// PluginRuntimeBuiltin 表示内置插件
	PluginRuntimeBuiltin PluginRuntime = "Builtin"
	// PluginRuntimeExternal 表示外部进程插件
	PluginRuntimeExternal PluginRuntime = "External"
	// PluginRuntimeWasm 表示 Wasm 插件
	PluginRuntimeWasm PluginRuntime = "Wasm"
)

// PluginPhase 表示插件执行阶段
type PluginPhase string

const (
	// PluginPhaseRequestHeaders 表示请求头阶段
	PluginPhaseRequestHeaders PluginPhase = "RequestHeaders"
	// PluginPhaseRequestBody 表示请求体阶段
	PluginPhaseRequestBody PluginPhase = "RequestBody"
	// PluginPhaseBeforeAIRoute 表示 AI 路由选择前阶段
	PluginPhaseBeforeAIRoute PluginPhase = "BeforeAIRoute"
	// PluginPhaseBeforeProviderCall 表示调用模型供应商前阶段
	PluginPhaseBeforeProviderCall PluginPhase = "BeforeProviderCall"
	// PluginPhaseResponseHeaders 表示响应头阶段
	PluginPhaseResponseHeaders PluginPhase = "ResponseHeaders"
	// PluginPhaseResponseBody 表示响应体阶段
	PluginPhaseResponseBody PluginPhase = "ResponseBody"
	// PluginPhaseStreamChunk 表示流式响应片段阶段
	PluginPhaseStreamChunk PluginPhase = "StreamChunk"
	// PluginPhaseUsage 表示用量统计阶段
	PluginPhaseUsage PluginPhase = "Usage"
	// PluginPhaseError 表示错误处理阶段
	PluginPhaseError PluginPhase = "Error"
)

// PluginFailurePolicy 表示插件失败处理策略
type PluginFailurePolicy string

const (
	// PluginFailurePolicyFailClose 表示插件失败时拒绝请求
	PluginFailurePolicyFailClose PluginFailurePolicy = "FailClose"
	// PluginFailurePolicyFailOpen 表示插件失败时放行请求
	PluginFailurePolicyFailOpen PluginFailurePolicy = "FailOpen"
	// PluginFailurePolicySkipAndLog 表示插件失败时跳过并记录日志
	PluginFailurePolicySkipAndLog PluginFailurePolicy = "SkipAndLog"
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
	// +listType=atomic
	Gateways []Gateway `json:"gateways"`
	// +listType=atomic
	Routes []Route `json:"routes"`
	// +listType=atomic
	AIRoutes []AIRoute `json:"aiRoutes"`
	// +listType=atomic
	Upstreams []Upstream `json:"upstreams"`
	// +listType=atomic
	AIProviders []AIProvider `json:"aiProviders"`
	// +listType=atomic
	AIModels []AIModel `json:"aiModels"`
	// +listType=atomic
	AIPolicies []AIPolicy `json:"aiPolicies"`
	// +listType=atomic
	Plugins []Plugin `json:"plugins"`
	// +listType=atomic
	AuthPolicies []AuthPolicy `json:"authPolicies"`
	// +listType=atomic
	RateLimitPolicies []RateLimitPolicy `json:"rateLimitPolicies"`
	// +listType=atomic
	PolicyBindings []PolicyBinding `json:"policyBindings"`
	// +listType=atomic
	PluginBindings []PluginBinding `json:"pluginBindings"`
}

// ResourceStatus 表示声明式资源状态
type ResourceStatus struct {
	// +listType=atomic
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RuntimeGroup 表示一组数据面运行时的逻辑投放单元
type RuntimeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RuntimeGroupSpec `json:"spec,omitempty"`
	Status ResourceStatus   `json:"status,omitempty"`
}

// RuntimeGroupList 表示 RuntimeGroup 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RuntimeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RuntimeGroup `json:"items"`
}

// RuntimeGroupSpec 定义一组数据面运行时的 target 和控制台展示信息
type RuntimeGroupSpec struct {
	// DisplayName 保存控制台展示名称，不参与引用匹配
	DisplayName string `json:"displayName,omitempty"`
	// Description 保存运维识别用的说明，不参与运行时匹配
	Description string `json:"description,omitempty"`
	// Enabled 表示该运行组是否允许承载新的 Gateway 配置
	Enabled bool `json:"enabled"`
	// TargetRef 表示该运行组对应的运行时 target
	TargetRef TargetRef `json:"targetRef"`
}

// TargetRef 引用一个运行时 target
type TargetRef struct {
	Name string `json:"name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RuntimeSnapshot 表示 controller 编译后交给运行时 target 的配置快照
type RuntimeSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RuntimeSnapshotSpec `json:"spec,omitempty"`
}

// RuntimeSnapshotList 表示 RuntimeSnapshot 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RuntimeSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RuntimeSnapshot `json:"items"`
}

// RuntimeSnapshotSpec 定义某个 target 可消费的网关配置快照
type RuntimeSnapshotSpec struct {
	Target  string               `json:"target"`
	Gateway string               `json:"gateway"`
	Version string               `json:"version"`
	Config  runtime.RawExtension `json:"config"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Gateway 声明一个流量入口
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

// GatewaySpec 定义 Gateway 的入口监听、运行组和域名绑定
type GatewaySpec struct {
	// DisplayName 保存控制台展示名称，不参与引用和运行时匹配
	DisplayName string `json:"displayName,omitempty"`
	// Description 保存控制台展示和运维识别用的说明，不参与运行时匹配
	Description string `json:"description,omitempty"`
	// Enabled 表示 Gateway 是否参与编译和下发
	Enabled bool `json:"enabled"`
	// RuntimeGroupRef 表示 Gateway 绑定的数据面运行组
	RuntimeGroupRef RuntimeGroupRef `json:"runtimeGroupRef,omitempty"`
	// +listType=atomic
	Listeners []Listener `json:"listeners"`
	// +listType=atomic
	HostBindings []HostBinding `json:"hostBindings,omitempty"`
}

// RuntimeGroupRef 引用一个数据面运行组
type RuntimeGroupRef struct {
	Name string `json:"name"`
}

// Protocol 表示网关资源中可声明的流量协议
type Protocol string

const (
	// ProtocolHTTP 表示普通 HTTP 流量
	ProtocolHTTP Protocol = "HTTP"
	// ProtocolHTTPS 表示 HTTPS 流量
	ProtocolHTTPS Protocol = "HTTPS"
)

// Listener 声明一个 Gateway 监听端口
type Listener struct {
	Name     string   `json:"name"`
	Protocol Protocol `json:"protocol"`
	Port     int      `json:"port"`
}

// HostBinding 声明 Host 到 Gateway 监听器的绑定关系
type HostBinding struct {
	Hostname string `json:"hostname,omitempty"`
	// +listType=atomic
	ListenerRefs []string    `json:"listenerRefs,omitempty"`
	TLS          *GatewayTLS `json:"tls,omitempty"`
}

// GatewayTLS 声明域名绑定使用的 TLS 证书引用
type GatewayTLS struct {
	CertificateRef string `json:"certificateRef,omitempty"`
}

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AIRoute 声明 AI 请求匹配规则和模型供应商引用
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
	// +listType=atomic
	ParentRefs []string `json:"parentRefs"`
	// +listType=atomic
	Hostnames  []string `json:"hostnames,omitempty"`
	Path       string   `json:"path,omitempty"`
	PathPrefix string   `json:"pathPrefix"`
	Model      string   `json:"model"`
	// +listType=atomic
	Models []AIModelRef `json:"models,omitempty"`
	// +listType=atomic
	ProviderRefs []AIProviderRef `json:"providerRefs"`
	// +listType=atomic
	PolicyRefs []string `json:"policyRefs,omitempty"`
}

// AIModelRef 表示 AIRoute 中的 AIModel 引用
type AIModelRef struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// AIProviderRef 表示 AIRoute 中的 AIProvider 引用
type AIProviderRef struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Upstream 声明一个逻辑上游服务
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

// UpstreamSpec 定义 Upstream 的展示信息和端点集合
type UpstreamSpec struct {
	// DisplayName 保存控制台展示名称，不参与引用匹配
	DisplayName string `json:"displayName,omitempty"`
	// +listType=atomic
	Endpoints []Endpoint `json:"endpoints"`
}

const (
	// AnnotationUpstreamServiceType 保存控制台展示的服务类型
	AnnotationUpstreamServiceType = "upstream.ingate.io/service-type"
	// AnnotationUpstreamLoadBalancePolicy 保存控制台维护的负载均衡策略
	AnnotationUpstreamLoadBalancePolicy = "upstream.ingate.io/load-balance-policy"
	// AnnotationUpstreamEndpoints 保存控制台维护的端点启停和权重信息
	AnnotationUpstreamEndpoints = "upstream.ingate.io/endpoints"
	// AnnotationUpstreamHealthCheck 保存控制台维护的健康检查配置
	AnnotationUpstreamHealthCheck = "upstream.ingate.io/health-check"
)

// Endpoint 声明一个上游端点
type Endpoint struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AIProvider 声明一个 AI 模型供应商
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
	// +listType=atomic
	Models []string `json:"models"`
	// CredentialRef 引用后续 Credential/Secret 资源
	CredentialRef string `json:"credentialRef,omitempty"`
	// +listType=atomic
	Headers       []HeaderPair `json:"headers,omitempty"`
	TimeoutMillis int          `json:"timeoutMillis,omitempty"`
}

// HeaderPair 表示要注入到上游请求的 header
type HeaderPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AIModel 声明一个可被 AIRoute 引用的模型
type AIModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIModelSpec    `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// AIModelList 表示 AIModel 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AIModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AIModel `json:"items"`
}

// AIModelSpec 定义对外模型和供应商模型的映射
type AIModelSpec struct {
	ProviderRef   string `json:"providerRef"`
	ProviderModel string `json:"providerModel"`
	// +listType=atomic
	Capabilities []string `json:"capabilities,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AIPolicy 声明 AI 请求策略
type AIPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIPolicySpec   `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// AIPolicyList 表示 AIPolicy 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AIPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AIPolicy `json:"items"`
}

// AIPolicySpec 定义 AI 请求策略配置
type AIPolicySpec struct {
	ExecutionTarget AIExecutionTargetType `json:"executionTarget,omitempty"`
	TimeoutMillis   int                   `json:"timeoutMillis,omitempty"`
	Retry           AIRetryPolicy         `json:"retry,omitempty"`
	Fallback        AIFallbackPolicy      `json:"fallback,omitempty"`
	Usage           AIUsagePolicy         `json:"usage,omitempty"`
}

// AIRetryPolicy 定义 AI 请求重试策略
type AIRetryPolicy struct {
	Attempts int `json:"attempts,omitempty"`
}

// AIFallbackPolicy 定义 AI 请求 fallback 策略
type AIFallbackPolicy struct {
	Enabled bool `json:"enabled,omitempty"`
	// +listType=atomic
	Models []string `json:"models,omitempty"`
}

// AIUsagePolicy 定义 AI 用量采集策略
type AIUsagePolicy struct {
	Enabled bool `json:"enabled,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Plugin 声明一个可绑定到网关资源的插件
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
	// +listType=atomic
	Phases []PluginPhase `json:"phases,omitempty"`
	// +listType=atomic
	TargetKinds   []Kind              `json:"targetKinds,omitempty"`
	FailurePolicy PluginFailurePolicy `json:"failurePolicy,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuthPolicy 声明认证策略
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RateLimitPolicy 声明限流策略
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PluginBinding 声明一组插件绑定到哪个资源
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
	TargetRef     PluginTargetRef     `json:"targetRef"`
	Phase         PluginPhase         `json:"phase,omitempty"`
	Priority      int                 `json:"priority,omitempty"`
	FailurePolicy PluginFailurePolicy `json:"failurePolicy,omitempty"`
	// +listType=atomic
	Plugins []PluginRef `json:"plugins"`
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyBinding 声明一组策略绑定到哪个资源
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
	// +listType=atomic
	Policies []PolicyRef `json:"policies"`
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
