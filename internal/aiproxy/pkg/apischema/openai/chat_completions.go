// Package openai 定义 AI Proxy 当前使用的 OpenAI 协议字段。
// 请求类型只提取所需字段，错误类型只编码本地响应，不用于重建上游消息。
package openai

// ChatCompletionRequest 表达 Chat Completions 当前需要读取的请求字段。
// 原始 JSON 由调用方保留，未声明的提供商字段仍通过原始请求体传递。
type ChatCompletionRequest struct {
	Model string `json:"model"`
}
