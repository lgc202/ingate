# AI Proxy 插件

AI Proxy 是 Ingate 内置的数据面插件。当前对外只处理 OpenAI-compatible
`POST /v1/chat/completions`，模型服务仍由声明式 `Upstream` 和 Route 的
`modelRouting` 管理，用户不会直接编辑插件私有 JSON。

一条模型 Route 可以按请求体中的公开 `model` 选择不同的模型 Upstream。插件读取完整
请求体后完成别名匹配，按目标协议转换请求体和上游路径，写入 Controller 生成的受控
规则版本 Header、Cluster Header 和认证 Header。Envoy 按标准 Route cache 语义重新选中
只接受内部 Header 的续接 Route，由续接 Route 选择 Cluster、写入上游 Host 并移除内部 Header。

当前支持 OpenAI、DeepSeek、通义千问兼容模式、Anthropic、Gemini 和自定义
OpenAI-compatible 服务。普通响应、错误、Token usage 和 SSE 会转换成
OpenAI-compatible 结构，响应中的 `model` 始终使用客户端公开别名。客户端提供的内部
选路 Header 和模型服务凭据会在处理开始时移除。
受 Token 配额保护的 OpenAI-compatible 流式 Route 会由插件内部注入
`stream_options.include_usage=true`，以便响应结束后按实际 usage 记账。该字段不对
客户端开放，也不会注入 Anthropic 或 Gemini 上游。

协议转换由顶层 `pkg/llm` 提供。该包不依赖 Proxy-Wasm；本目录只承载配置索引、请求
生命周期和 hostcall 适配。当前只支持文本 `system`、`user`、`assistant` 消息和
`model/messages/stream/temperature/top_p/max_tokens/stop`，不支持 Tools、多模态、
fallback 或模型级重试。

单次请求体上限为 1 MiB，插件默认发布到 `/opt/ingate/plugins/ai-proxy.wasm`。
