package aidataplane

const (
	// ListenerPortHeader 标识 Envoy 已匹配的公共 Listener 端口。
	// 仅由受保护的 Go Unix 入口采信；Envoy 必须覆盖客户端提供的值。
	ListenerPortHeader = "x-ingate-ai-listener-port"
	// HostNameHeader 标识 Envoy 已匹配的 Host 完整编译名称，不是客户端 Host。
	// 与 ListenerPortHeader 一起定位入口，不授予访问权限。
	HostNameHeader = "x-ingate-ai-host"
)
