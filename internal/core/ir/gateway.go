// Package ir 定义运行时无关的网关编译结果
package ir

// LogicalGateway 表示一个 Gateway 编译后的运行时无关模型
type LogicalGateway struct {
	Name      string
	Listeners []LogicalListener
	Routes    []LogicalRoute
	Upstreams []LogicalUpstream
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
	PathPrefix string
	Methods    []string
	Headers    []LogicalHeaderMatch
	Upstreams  []LogicalUpstreamRef
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
