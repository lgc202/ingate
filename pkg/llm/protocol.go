package llm

// Protocol 表示模型服务实际使用的请求和响应协议
type Protocol string

const (
	// ProtocolOpenAIChatCompletions 表示 OpenAI Chat Completions 协议
	ProtocolOpenAIChatCompletions Protocol = "OpenAI"
	// ProtocolAnthropicMessages 表示 Anthropic Messages 协议
	ProtocolAnthropicMessages Protocol = "Anthropic"
	// ProtocolGeminiGenerateContent 表示 Gemini generateContent 协议
	ProtocolGeminiGenerateContent Protocol = "Gemini"
)
