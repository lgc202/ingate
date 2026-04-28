// Package ir 定义运行时无关的网关编译结果
package ir

import "github.com/lgc202/ingate-next/internal/core/resource"

// LogicalGateway 表示一个 Gateway 编译后的运行时无关模型
type LogicalGateway struct {
	Name           string
	Listeners      []LogicalListener
	Routes         []LogicalRoute
	Upstreams      []LogicalUpstream
	AuthPolicies   []LogicalAuthPolicy
	PolicyBindings []LogicalPolicyBinding
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
