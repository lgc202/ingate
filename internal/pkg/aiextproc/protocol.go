// Package aiextproc 定义 Controller、AI ExtProc、Envoy 和 ALS 共享的内部执行协议
package aiextproc

const (
	// MetadataNamespace 隔离 Ingate 写入 Envoy 的 AI 执行元数据
	MetadataNamespace = "ingate.ai"
	// AttributeNamespace 是 Envoy 向上游 ExtProc 传递 xDS 属性的固定命名空间
	AttributeNamespace = "envoy.filters.http.ext_proc"

	// RequestIDHeader 关联同一请求的 downstream ExtProc 流和 upstream ExtProc 流
	RequestIDHeader = "x-ingate-ai-request-id"
	// ModelHeader 把请求体中的客户端模型转换为 Envoy 可以匹配的内部 Header
	ModelHeader = "x-ingate-ai-model"
	// UpstreamModelHeader 由 Envoy 在选中模型线路后写入实际模型名
	UpstreamModelHeader = "x-ingate-ai-upstream-model"

	// ClientHostField 保存进入网关时的请求 Host，避免上游改写覆盖客户端请求信息
	ClientHostField = "client_host"
	// ClientPathField 保存进入网关时不含查询参数的请求路径
	ClientPathField = "client_path"
	// ServiceIDField 记录 Envoy 最终选择的模型 Service ID
	ServiceIDField = "service_id"
	// ServiceProtocolField 记录模型 Service 使用的厂商协议
	ServiceProtocolField = "protocol"
	// ClientModelField 记录 AI Route 对外发布的客户端模型名
	ClientModelField = "client_model"
	// UpstreamModelField 记录模型 Service 实际接收的模型名
	UpstreamModelField = "upstream_model"
	// UpstreamProtocolField 记录模型 Service 使用的厂商协议
	UpstreamProtocolField = "upstream_protocol"
	// ResponseModelField 记录模型服务响应中的真实模型名
	ResponseModelField = "response_model"
	// FinishReasonField 记录模型响应结束原因
	FinishReasonField = "finish_reason"
	// InputTokensField 记录归一化后的输入 Token 数量
	InputTokensField = "input_tokens"
	// OutputTokensField 记录归一化后的输出 Token 数量
	OutputTokensField = "output_tokens"
	// TotalTokensField 记录归一化后的总 Token 数量
	TotalTokensField = "total_tokens"
	// ServiceIDAttribute 读取 Envoy 最终选择的模型 Service ID
	ServiceIDAttribute = "xds.upstream_host_metadata.filter_metadata['" + MetadataNamespace + "']['" + ServiceIDField + "']"
	// ServiceProtocolAttribute 读取模型 Service 使用的厂商协议
	ServiceProtocolAttribute = "xds.upstream_host_metadata.filter_metadata['" + MetadataNamespace + "']['" + ServiceProtocolField + "']"
)

// UpstreamProtocol 表示模型 Service 实际使用的 HTTP API 协议
type UpstreamProtocol string

const (
	// UpstreamProtocolOpenAI 保持 OpenAI Chat Completions 请求与响应格式
	UpstreamProtocolOpenAI UpstreamProtocol = "openai"
	// UpstreamProtocolAnthropic 把 OpenAI Chat Completions 转换为 Anthropic Messages
	UpstreamProtocolAnthropic UpstreamProtocol = "anthropic"
)
