package openai

// FinishReason 表示 OpenAI-compatible 结束原因
type FinishReason string

const (
	// FinishReasonStop 表示模型自然结束或命中停止序列
	FinishReasonStop FinishReason = "stop"
	// FinishReasonLength 表示达到最大输出 Token 数
	FinishReasonLength FinishReason = "length"
	// FinishReasonContentFilter 表示内容被上游安全策略阻断
	FinishReasonContentFilter FinishReason = "content_filter"
)

const (
	// ObjectChatCompletion 是普通 Chat Completions 响应的对象类型
	ObjectChatCompletion = "chat.completion"
	// ObjectChatCompletionChunk 是流式 Chat Completions 响应的对象类型
	ObjectChatCompletionChunk = "chat.completion.chunk"
)

// Usage 表示统一后的 Token 用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletion 表示统一后的 OpenAI-compatible 普通响应
type ChatCompletion struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   *Usage             `json:"usage,omitempty"`
}

// CompletionChoice 表示普通响应中的一个候选结果
type CompletionChoice struct {
	Index        int             `json:"index"`
	Message      ResponseMessage `json:"message"`
	FinishReason *FinishReason   `json:"finish_reason"`
}

// ResponseMessage 表示统一后的模型文本回复
type ResponseMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionChunk 表示统一后的 OpenAI-compatible 流式响应块
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// ChunkChoice 表示流式响应中的一个候选增量
type ChunkChoice struct {
	Index        int           `json:"index"`
	Delta        MessageDelta  `json:"delta"`
	FinishReason *FinishReason `json:"finish_reason"`
}

// MessageDelta 表示流式响应中的文本增量
type MessageDelta struct {
	Role    Role    `json:"role,omitempty"`
	Content *string `json:"content,omitempty"`
}
