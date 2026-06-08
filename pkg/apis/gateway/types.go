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
	// KindRateLimitPolicy 表示 RateLimitPolicy 资源类型
	KindRateLimitPolicy Kind = "RateLimitPolicy"
	// KindRedisStore 表示 RedisStore 资源类型
	KindRedisStore Kind = "RedisStore"
	// KindPolicyBinding 表示 PolicyBinding 资源类型
	KindPolicyBinding Kind = "PolicyBinding"
)

// RateLimitMode 表示限流计数状态的存放位置
type RateLimitMode string

const (
	// RateLimitModeLocal 表示每个数据面实例独立计数
	RateLimitModeLocal RateLimitMode = "Local"
	// RateLimitModeGlobal 表示通过 Redis 共享计数
	RateLimitModeGlobal RateLimitMode = "Global"
)

// RateLimitKeyType 表示限流 key 的组成维度
type RateLimitKeyType string

const (
	// RateLimitKeyTypeIP 表示按客户端 IP 生成限流 key
	RateLimitKeyTypeIP RateLimitKeyType = "IP"
	// RateLimitKeyTypeHeader 表示按请求 header 生成限流 key
	RateLimitKeyTypeHeader RateLimitKeyType = "Header"
	// RateLimitKeyTypeQuery 表示按 query 参数生成限流 key
	RateLimitKeyTypeQuery RateLimitKeyType = "Query"
	// RateLimitKeyTypeCookie 表示按 cookie 生成限流 key
	RateLimitKeyTypeCookie RateLimitKeyType = "Cookie"
	// RateLimitKeyTypeConsumer 表示按认证后的 consumer 生成限流 key
	RateLimitKeyTypeConsumer RateLimitKeyType = "Consumer"
	// RateLimitKeyTypeRoute 表示按 Route 生成限流 key
	RateLimitKeyTypeRoute RateLimitKeyType = "Route"
	// RateLimitKeyTypeGateway 表示按 Gateway 生成限流 key
	RateLimitKeyTypeGateway RateLimitKeyType = "Gateway"
	// RateLimitKeyTypeRouteRule 表示按 RouteRule 生成限流 key
	RateLimitKeyTypeRouteRule RateLimitKeyType = "RouteRule"
	// RateLimitKeyTypeJWTClaim 表示按 JWT claim 生成限流 key
	RateLimitKeyTypeJWTClaim RateLimitKeyType = "JWTClaim"
	// RateLimitKeyTypeAPIKey 表示按 API Key 生成限流 key
	RateLimitKeyTypeAPIKey RateLimitKeyType = "APIKey"
	// RateLimitKeyTypeTenant 表示按租户生成限流 key
	RateLimitKeyTypeTenant RateLimitKeyType = "Tenant"
)

// RateLimitAlgorithm 表示限流计数算法
type RateLimitAlgorithm string

const (
	// RateLimitAlgorithmFixedWindow 表示固定窗口限流
	RateLimitAlgorithmFixedWindow RateLimitAlgorithm = "FixedWindow"
	// RateLimitAlgorithmSlidingWindow 表示滑动窗口限流
	RateLimitAlgorithmSlidingWindow RateLimitAlgorithm = "SlidingWindow"
	// RateLimitAlgorithmTokenBucket 表示令牌桶限流
	RateLimitAlgorithmTokenBucket RateLimitAlgorithm = "TokenBucket"
)

// RateLimitFailurePolicy 表示限流执行异常时的处理方式
type RateLimitFailurePolicy string

const (
	// RateLimitFailurePolicyFailOpen 表示限流执行失败时放行请求
	RateLimitFailurePolicyFailOpen RateLimitFailurePolicy = "FailOpen"
	// RateLimitFailurePolicyFailClose 表示限流执行失败时拒绝请求
	RateLimitFailurePolicyFailClose RateLimitFailurePolicy = "FailClose"
)

// RedisMode 表示 Redis 部署模式
type RedisMode string

const (
	// RedisModeStandalone 表示单实例 Redis
	RedisModeStandalone RedisMode = "Standalone"
	// RedisModeSentinel 表示 Redis Sentinel
	RedisModeSentinel RedisMode = "Sentinel"
	// RedisModeCluster 表示 Redis Cluster
	RedisModeCluster RedisMode = "Cluster"
)

// Bundle 表示一次编译所需的资源集合
type Bundle struct {
	// +listType=atomic
	Gateways []Gateway `json:"gateways"`
	// +listType=atomic
	Routes []Route `json:"routes"`
	// +listType=atomic
	Upstreams []Upstream `json:"upstreams"`
	// +listType=atomic
	RateLimitPolicies []RateLimitPolicy `json:"rateLimitPolicies"`
	// +listType=atomic
	RedisStores []RedisStore `json:"redisStores"`
	// +listType=atomic
	PolicyBindings []PolicyBinding `json:"policyBindings"`
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

// UpstreamType 表示 Upstream 的业务分类
type UpstreamType string

const (
	// UpstreamTypeApplication 表示普通应用服务
	UpstreamTypeApplication UpstreamType = "application"
	// UpstreamTypeModel 表示模型服务
	UpstreamTypeModel UpstreamType = "model"
	// UpstreamTypeAgent 表示 Agent 服务
	UpstreamTypeAgent UpstreamType = "agent"
	// UpstreamTypeMCP 表示 MCP 服务
	UpstreamTypeMCP UpstreamType = "mcp"
)

// UpstreamLoadBalancePolicy 表示 Upstream 的负载均衡策略
type UpstreamLoadBalancePolicy string

const (
	// UpstreamLoadBalancePolicyRoundRobin 表示轮询
	UpstreamLoadBalancePolicyRoundRobin UpstreamLoadBalancePolicy = "round_robin"
	// UpstreamLoadBalancePolicyLeastRequest 表示最少请求
	UpstreamLoadBalancePolicyLeastRequest UpstreamLoadBalancePolicy = "least_request"
	// UpstreamLoadBalancePolicyRandom 表示随机
	UpstreamLoadBalancePolicyRandom UpstreamLoadBalancePolicy = "random"
)

// UpstreamSpec 定义 Upstream 的展示信息、流量策略和端点集合
type UpstreamSpec struct {
	// DisplayName 保存控制台展示名称，不参与引用匹配
	DisplayName string `json:"displayName,omitempty"`
	// Type 保存 Upstream 的业务分类，用于区分普通服务、模型服务和 Agent/MCP 服务
	Type UpstreamType `json:"type,omitempty"`
	// LoadBalancePolicy 指定多个端点之间的负载均衡策略
	LoadBalancePolicy UpstreamLoadBalancePolicy `json:"loadBalancePolicy,omitempty"`
	// HealthCheck 描述 Upstream 端点的主动健康检查配置
	HealthCheck *UpstreamHealthCheck `json:"healthCheck,omitempty"`
	// +listType=atomic
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint 声明一个上游端点
type Endpoint struct {
	// Name 是端点在 Upstream 内的稳定标识
	Name    string `json:"name,omitempty"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	// Weight 是端点参与负载均衡时的相对权重，取值范围为 1-100
	Weight int `json:"weight,omitempty"`
	// Enabled 控制端点是否参与运行时配置生成
	Enabled bool `json:"enabled"`
}

// UpstreamHealthCheck 声明 Upstream 的主动健康检查配置
type UpstreamHealthCheck struct {
	Enabled         bool   `json:"enabled"`
	Path            string `json:"path,omitempty"`
	IntervalSeconds int    `json:"intervalSeconds,omitempty"`
	TimeoutSeconds  int    `json:"timeoutSeconds,omitempty"`
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
	DisplayName string        `json:"displayName"`
	Description string        `json:"description,omitempty"`
	Enabled     bool          `json:"enabled"`
	Mode        RateLimitMode `json:"mode"`
	// +listType=atomic
	Rules         []RateLimitRule        `json:"rules"`
	Global        *GlobalRateLimitConfig `json:"global,omitempty"`
	Response      RateLimitResponse      `json:"response,omitempty"`
	FailurePolicy RateLimitFailurePolicy `json:"failurePolicy,omitempty"`
}

// RateLimitRule 定义一条限流规则
type RateLimitRule struct {
	Name      string             `json:"name"`
	Key       RateLimitKey       `json:"key"`
	Limit     RateLimitQuota     `json:"limit"`
	Algorithm RateLimitAlgorithm `json:"algorithm,omitempty"`
}

// RateLimitKey 定义限流计数 key
type RateLimitKey struct {
	// +listType=atomic
	Parts []RateLimitKeyPart `json:"parts"`
}

// RateLimitKeyPart 定义限流 key 的一个组成部分
type RateLimitKeyPart struct {
	Type RateLimitKeyType `json:"type"`
	Name string           `json:"name,omitempty"`
}

// RateLimitQuota 定义限流额度
type RateLimitQuota struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
	Burst         int `json:"burst,omitempty"`
}

// GlobalRateLimitConfig 定义 Redis-backed global limit 配置
type GlobalRateLimitConfig struct {
	RedisRef      string `json:"redisRef"`
	Prefix        string `json:"prefix,omitempty"`
	TimeoutMillis int    `json:"timeoutMillis,omitempty"`
}

// RateLimitResponse 定义超限响应
type RateLimitResponse struct {
	StatusCode         int    `json:"statusCode,omitempty"`
	Message            string `json:"message,omitempty"`
	QuotaHeaderEnabled bool   `json:"quotaHeaderEnabled,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RedisStore 声明 Redis 连接配置
type RedisStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisStoreSpec `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// RedisStoreList 表示 RedisStore 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RedisStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RedisStore `json:"items"`
}

// RedisStoreSpec 定义 Redis 连接配置
type RedisStoreSpec struct {
	DisplayName string    `json:"displayName"`
	Description string    `json:"description,omitempty"`
	Mode        RedisMode `json:"mode"`
	Address     string    `json:"address"`
	// +listType=atomic
	Addresses            []string `json:"addresses,omitempty"`
	DB                   int      `json:"db,omitempty"`
	TLS                  bool     `json:"tls,omitempty"`
	TLSServerName        string   `json:"tlsServerName,omitempty"`
	Username             string   `json:"username,omitempty"`
	PasswordRef          string   `json:"passwordRef,omitempty"`
	ConnectTimeoutMillis int      `json:"connectTimeoutMillis,omitempty"`
	CommandTimeoutMillis int      `json:"commandTimeoutMillis,omitempty"`
	PoolSize             int      `json:"poolSize,omitempty"`
	MinIdleConns         int      `json:"minIdleConns,omitempty"`
	SentinelMaster       string   `json:"sentinelMaster,omitempty"`
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
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Enabled     bool            `json:"enabled"`
	TargetRef   PolicyTargetRef `json:"targetRef"`
	// +listType=atomic
	Policies []PolicyRef `json:"policies"`
}

// PolicyTargetRef 表示策略绑定目标资源
type PolicyTargetRef struct {
	Kind     Kind   `json:"kind"`
	Name     string `json:"name"`
	RuleName string `json:"ruleName,omitempty"`
}

// PolicyRef 表示被绑定的策略资源引用
type PolicyRef struct {
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}
