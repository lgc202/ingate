// Package resource 定义声明式控制面资源
package resource

// Bundle 表示一次内存编译所需的资源集合
type Bundle struct {
	Gateways  []Gateway
	Routes    []Route
	Upstreams []Upstream
}

// Metadata 标识一个声明式资源
type Metadata struct {
	Name string
}

// Gateway 声明一个流量入口
type Gateway struct {
	Metadata Metadata
	Spec     GatewaySpec
}

// GatewaySpec 定义 Gateway 的监听器
type GatewaySpec struct {
	Listeners []Listener
}

// Listener 声明一个 Gateway 监听端口
type Listener struct {
	Name     string
	Protocol string
	Port     int
	Hostname string
}

// Route 声明请求匹配规则和 Upstream 引用
type Route struct {
	Metadata Metadata
	Spec     RouteSpec
}

// RouteSpec 定义 Route 如何挂载到 Gateway
type RouteSpec struct {
	ParentRefs []string
	Hostnames  []string
	Rules      []RouteRule
}

// RouteRule 声明一条路由匹配规则和加权 Upstream 集合
type RouteRule struct {
	PathPrefix   string
	UpstreamRefs []UpstreamRef
}

// UpstreamRef 表示 RouteRule 中的 Upstream 引用
type UpstreamRef struct {
	Name   string
	Weight int
}

// Upstream 声明一个逻辑上游服务
type Upstream struct {
	Metadata Metadata
	Spec     UpstreamSpec
}

// UpstreamSpec 定义 Upstream 的端点集合
type UpstreamSpec struct {
	Endpoints []Endpoint
}

// Endpoint 声明一个上游端点
type Endpoint struct {
	Address string
	Port    int
}
