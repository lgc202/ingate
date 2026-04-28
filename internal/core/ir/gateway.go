// Package ir 定义运行时无关的网关编译结果
package ir

import "github.com/lgc202/ingate-next/internal/core/resource"

// LogicalGateway 表示一个 Gateway 编译后的运行时无关模型
type LogicalGateway struct {
	Name              string
	Listeners         []LogicalListener
	Routes            []LogicalRoute
	AIRoutes          []LogicalAIRoute
	Upstreams         []LogicalUpstream
	AIProviders       []LogicalAIProvider
	Plugins           []LogicalPlugin
	AuthPolicies      []LogicalAuthPolicy
	RateLimitPolicies []LogicalRateLimitPolicy
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
	PathPrefix    string
	Methods       []string
	TimeoutMillis int
	Headers       []LogicalHeaderMatch
	Upstreams     []LogicalUpstreamRef
}

// LogicalHeaderMatch 表示编译后的 HTTP header 精确匹配条件
type LogicalHeaderMatch struct {
	Name  string
	Value string
}

// LogicalUpstreamRef 表示已解析的 Upstream 引用
type LogicalUpstreamRef struct {
	Name   string
	Weight int
}

// LogicalAIRoute 表示编译后的 AI 路由
type LogicalAIRoute struct {
	Name       string
	PathPrefix string
	Model      string
	Providers  []LogicalAIProviderRef
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
	Requests      int
	WindowSeconds int
	KeyBy         resource.RateLimitKey
	Header        string
}

// LogicalPolicyBinding 表示编译后的策略绑定关系
type LogicalPolicyBinding struct {
	Name     string
	Target   LogicalPolicyTarget
	Policies []LogicalPolicyRef
}

// LogicalPolicyTarget 表示策略绑定目标
type LogicalPolicyTarget struct {
	Kind resource.Kind
	Name string
}

// LogicalPolicyRef 表示被绑定的策略引用
type LogicalPolicyRef struct {
	Kind resource.Kind
	Name string
}

// LogicalPluginBinding 表示编译后的插件绑定关系
type LogicalPluginBinding struct {
	Name    string
	Target  LogicalPluginTarget
	Plugins []LogicalPluginRef
}

// LogicalPluginTarget 表示插件绑定目标
type LogicalPluginTarget struct {
	Kind resource.Kind
	Name string
}

// LogicalPluginRef 表示被绑定的插件引用
type LogicalPluginRef struct {
	Name   string
	Config map[string]any
}
