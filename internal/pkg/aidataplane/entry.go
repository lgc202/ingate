package aidataplane

const (
	// EntryProtocolHeader 标识 Envoy 已选择的模型或 MCP 入口，必须覆盖客户端值。
	EntryProtocolHeader = "x-ingate-ai-entry-protocol"
	// ListenerPortHeader 标识 Envoy 已匹配的公共 Listener 端口。
	// 仅由受保护的 Go Unix 入口采信；Envoy 必须覆盖客户端提供的值。
	ListenerPortHeader = "x-ingate-ai-listener-port"
	// HostNameHeader 标识 Envoy 已匹配的 Host 完整编译名称，不是客户端 Host。
	// 与 ListenerPortHeader 一起定位入口，不授予访问权限。
	HostNameHeader = "x-ingate-ai-host"
)

const (
	// ModelChatCompletionsPath 是 Go 入口固定注册的 Chat Completions 创建路径。
	ModelChatCompletionsPath = "/v1/chat/completions"
	// ModelResponsesPath 是 Go 入口固定注册的 Responses 创建路径。
	// Envoy 必须为两者生成同一精确路径；同一 Host 启用模型路由时，MCPRoute 不得与它们冲突。
	ModelResponsesPath = "/v1/responses"
)
