// Package policy 执行 ACL 插件的访问控制判断
package policy

import config "github.com/lgc202/ingate/pkg/plugin/acl"

// RouteKey 标识一条挂载访问控制策略的 Route
type RouteKey struct {
	GatewayName string
	RouteName   string
}

// Route 保存一次访问判断需要的策略和请求 Header
type Route struct {
	policies        []config.Policy
	RequiredHeaders []string
}

// Routes 保存 Listener 中按 Gateway 和 Route 建立的访问控制索引
type Routes map[RouteKey]Route

// RequestAttributes 表示 ACL 判断需要读取的请求属性
type RequestAttributes struct {
	RemoteAddr string
	Headers    map[string]string
}

// Decision 表示 ACL 对一次请求的访问控制结果
type Decision struct {
	Allowed    bool
	StatusCode int
	Message    string
}
