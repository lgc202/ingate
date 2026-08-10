// Package policy 执行客户端 IP 访问限制判断
package policy

import "net/netip"

// RouteKey 标识一条挂载 IP 访问限制策略的 Route
type RouteKey struct {
	GatewayName string
	RouteName   string
}

// Route 保存一条 Route 上已经解析的 IP 访问限制策略
type Route struct {
	policies []restriction
}

type restriction struct {
	allow []netip.Prefix
	deny  []netip.Prefix
}

// Routes 保存 Listener 中按 Gateway 和 Route 建立的 IP 访问限制索引
type Routes map[RouteKey]Route
