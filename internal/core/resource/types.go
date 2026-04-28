// Package resource 定义声明式控制面资源
package resource

// Bundle 表示一次内存编译所需的资源集合
type Bundle struct {
	Gateways  []Gateway  `json:"gateways"`
	Routes    []Route    `json:"routes"`
	Upstreams []Upstream `json:"upstreams"`
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
	PathPrefix   string        `json:"pathPrefix"`
	Methods      []string      `json:"methods"`
	Headers      []HeaderMatch `json:"headers"`
	UpstreamRefs []UpstreamRef `json:"upstreamRefs"`
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
