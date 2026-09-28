package domain

import "time"

// Route 表示面向产品接口的一条路由。
type Route struct {
	Name         string
	GatewayName  string
	ListenerName string
	Path         PathMatch
	UpstreamName string
	CreateTime   time.Time
	Version      string
}

// PathMatch 表示路径匹配方式和值。
type PathMatch struct {
	Type  string
	Value string
}
