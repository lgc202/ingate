// Package ir 定义运行时无关的网关编译结果
package ir

import (
	"encoding/json"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// LogicalGateway 表示一个 Gateway 编译后的运行时无关模型
type LogicalGateway struct {
	Name              string
	Listeners         []LogicalListener
	Routes            []LogicalRoute
	AIRoutes          []LogicalAIRoute
	Upstreams         []LogicalUpstream
	AIProviders       []LogicalAIProvider
	AIModels          []LogicalAIModel
	AIPolicies        []LogicalAIPolicy
	Plugins           []LogicalPlugin
	AuthPolicies      []LogicalAuthPolicy
	RateLimitPolicies []LogicalRateLimitPolicy
	RedisStores       []LogicalRedisStore
	PolicyBindings    []LogicalPolicyBinding
	PluginBindings    []LogicalPluginBinding
}

// LogicalListener 表示编译后的 Gateway 监听器
type LogicalListener struct {
	Name     string
	Protocol string
	Port     int
	Hostname string
}

// LogicalRoute 表示挂载到 Gateway 的编译后路由
type LogicalRoute struct {
	Name      string
	Hostnames []string
	Rules     []LogicalRouteRule
}

// LogicalRouteRule 表示编译后的路由规则
type LogicalRouteRule struct {
	Name                   string
	PathPrefix             string
	Methods                []string
	TimeoutMillis          int
	Headers                []LogicalHeaderMatch
	RequestHeadersToAdd    []LogicalHeaderValue
	RequestHeadersToRemove []string
	Retry                  LogicalRetryPolicy
	Upstreams              []LogicalUpstreamRef
}

// LogicalHeaderMatch 表示编译后的 HTTP header 精确匹配条件
type LogicalHeaderMatch struct {
	Name  string
	Value string
}

// LogicalHeaderValue 表示编译后的 HTTP header 写入动作
type LogicalHeaderValue struct {
	Name  string
	Value string
}

// LogicalRetryPolicy 表示编译后的路由失败重试策略
type LogicalRetryPolicy struct {
	Attempts            int
	PerTryTimeoutMillis int
}

// LogicalUpstreamRef 表示已解析的 Upstream 引用
type LogicalUpstreamRef struct {
	Name   string
	Weight int
}

// LogicalAIRoute 表示编译后的 AI 路由
type LogicalAIRoute struct {
	Name       string
	Hostnames  []string
	Path       string
	PathPrefix string
	Model      string
	Models     []LogicalAIModelRef
	Providers  []LogicalAIProviderRef
	PolicyRefs []string
}

// LogicalAIModelRef 表示已解析的 AIModel 引用
type LogicalAIModelRef struct {
	Name   string
	Weight int
}

// LogicalAIProviderRef 表示已解析的 AIProvider 引用
type LogicalAIProviderRef struct {
	Name   string
	Weight int
}

// LogicalUpstream 表示挂载路由实际使用到的编译后 Upstream
type LogicalUpstream struct {
	Name      string
	Endpoints []LogicalEndpoint
}

// LogicalEndpoint 表示编译后的 Upstream 端点
type LogicalEndpoint struct {
	Address string
	Port    int
}

// LogicalAIProvider 表示编译后的 AI 模型供应商
type LogicalAIProvider struct {
	Name     string
	Type     resource.AIProviderType
	Endpoint string
	Models   []string
}

// LogicalAIModel 表示编译后的 AI 模型映射
type LogicalAIModel struct {
	Name          string
	ProviderRef   string
	ProviderModel string
	Capabilities  []string
}

// LogicalAIPolicy 表示编译后的 AI 请求策略
type LogicalAIPolicy struct {
	Name            string
	ExecutionTarget resource.AIExecutionTargetType
	TimeoutMillis   int
	RetryAttempts   int
	FallbackEnabled bool
	FallbackModels  []string
	UsageEnabled    bool
}

// LogicalPlugin 表示编译后的插件声明
type LogicalPlugin struct {
	Name     string
	Runtime  resource.PluginRuntime
	Version  string
	Endpoint string
	Image    string
}

// LogicalAuthPolicy 表示编译后的认证策略
type LogicalAuthPolicy struct {
	Name   string
	Type   resource.AuthType
	APIKey LogicalAPIKeyAuth
}

// LogicalAPIKeyAuth 表示编译后的 API Key 认证配置
type LogicalAPIKeyAuth struct {
	Header string
	Query  string
}

// LogicalRateLimitPolicy 表示编译后的限流策略
type LogicalRateLimitPolicy struct {
	Name          string
	DisplayName   string
	Mode          resource.RateLimitMode
	Rules         []LogicalRateLimitRule
	Global        *LogicalGlobalRateLimit
	Response      LogicalRateLimitResponse
	FailurePolicy resource.RateLimitFailurePolicy
}

// LogicalRateLimitRule 表示编译后的单条限流规则
type LogicalRateLimitRule struct {
	Name      string
	Key       []LogicalRateLimitKeyPart
	Limit     LogicalRateLimitQuota
	Algorithm resource.RateLimitAlgorithm
}

// LogicalRateLimitKeyPart 表示编译后的限流 key 组成部分
type LogicalRateLimitKeyPart struct {
	Type resource.RateLimitKeyType
	Name string
}

// LogicalRateLimitQuota 表示编译后的限流额度
type LogicalRateLimitQuota struct {
	Requests      int
	WindowSeconds int
	Burst         int
}

// LogicalGlobalRateLimit 表示编译后的 Redis-backed global limit 配置
type LogicalGlobalRateLimit struct {
	RedisRef      string
	Prefix        string
	TimeoutMillis int
}

// LogicalRateLimitResponse 表示编译后的超限响应
type LogicalRateLimitResponse struct {
	StatusCode         int
	Message            string
	QuotaHeaderEnabled bool
}

// LogicalRedisStore 表示编译后的 Redis 连接配置
type LogicalRedisStore struct {
	Name                 string
	DisplayName          string
	Mode                 resource.RedisMode
	Address              string
	Addresses            []string
	DB                   int
	TLS                  bool
	TLSServerName        string
	Username             string
	PasswordRef          string
	ConnectTimeoutMillis int
	CommandTimeoutMillis int
	PoolSize             int
	MinIdleConns         int
	SentinelMaster       string
}

// LogicalPolicyBinding 表示编译后的策略绑定关系
type LogicalPolicyBinding struct {
	Name     string
	Target   LogicalPolicyTarget
	Policies []LogicalPolicyRef
}

// LogicalPolicyTarget 表示策略绑定目标
type LogicalPolicyTarget struct {
	Kind     resource.Kind
	Name     string
	RuleName string
}

// LogicalPolicyRef 表示被绑定的策略引用
type LogicalPolicyRef struct {
	Kind resource.Kind
	Name string
}

// LogicalPluginBinding 表示编译后的插件绑定关系
type LogicalPluginBinding struct {
	Name          string
	Target        LogicalPluginTarget
	Phase         resource.PluginPhase
	Priority      int
	FailurePolicy resource.PluginFailurePolicy
	Plugins       []LogicalPluginRef
}

// LogicalPluginTarget 表示插件绑定目标
type LogicalPluginTarget struct {
	Kind resource.Kind
	Name string
}

// LogicalPluginRef 表示被绑定的插件引用
type LogicalPluginRef struct {
	Name   string
	Config json.RawMessage
}
