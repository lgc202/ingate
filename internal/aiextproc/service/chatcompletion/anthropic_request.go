package chatcompletion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/tidwall/sjson"
)

const (
	// AnthropicMessagesPath 是 Anthropic Messages API 的标准请求路径。
	AnthropicMessagesPath = "/v1/messages"
	// AnthropicVersion 是当前转换器生成请求时使用的稳定 API 版本。
	AnthropicVersion = "2023-06-01"

	// defaultAnthropicMaxTokens 衔接 OpenAI 可选字段与 Anthropic 必填字段
	// 4096 与 Higress Claude 转换器保持一致，调用方显式配置时始终以调用方为准
	defaultAnthropicMaxTokens int64 = 4096
)

type openAIRequest struct {
	Model               string          `json:"model"`
	Messages            []openAIMessage `json:"messages"`
	MaxTokens           *int64          `json:"max_tokens"`
	MaxCompletionTokens *int64          `json:"max_completion_tokens"`
	Temperature         *float64        `json:"temperature"`
	TopP                *float64        `json:"top_p"`
	Stop                json.RawMessage `json:"stop"`
	Stream              *bool           `json:"stream"`
	N                   *int64          `json:"n"`
	Tools               json.RawMessage `json:"tools"`
	ToolChoice          json.RawMessage `json:"tool_choice"`
	ResponseFormat      json.RawMessage `json:"response_format"`
}

type openAIMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type openAIContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// RewriteAnthropicRequest 把 OpenAI Chat Completions 请求转换为 Anthropic Messages 请求
// 当前只接收文本消息和两种协议共有的采样参数，不能可靠转换的能力会明确拒绝。
func RewriteAnthropicRequest(body []byte, upstreamModel string) (UpstreamRequest, error) {
	if !json.Valid(body) {
		return UpstreamRequest{}, invalidRequest("request body must be valid JSON")
	}
	var source openAIRequest
	if err := json.Unmarshal(body, &source); err != nil {
		return UpstreamRequest{}, invalidRequest("request body does not match Chat Completions")
	}
	if source.Model == "" {
		return UpstreamRequest{}, invalidRequest("model must be a non-empty string")
	}
	if len(source.Messages) == 0 {
		return UpstreamRequest{}, invalidRequest("messages must not be empty")
	}
	if source.N != nil && *source.N != 1 {
		return UpstreamRequest{}, invalidRequest("anthropic upstream only supports n=1")
	}
	if hasJSONValue(source.Tools) || hasJSONValue(source.ToolChoice) {
		return UpstreamRequest{}, invalidRequest("tools are not supported by the anthropic upstream")
	}
	if hasJSONValue(source.ResponseFormat) {
		return UpstreamRequest{}, invalidRequest("response_format is not supported by the anthropic upstream")
	}

	messages, system, err := anthropicMessages(source.Messages)
	if err != nil {
		return UpstreamRequest{}, err
	}
	// 使用 Anthropic 官方 SDK 类型生成协议字段，减少手写厂商 JSON 带来的字段漂移
	params := anthropic.MessageNewParams{
		Model:     upstreamModel,
		Messages:  messages,
		System:    system,
		MaxTokens: defaultAnthropicMaxTokens,
	}
	// max_completion_tokens 是较新的 OpenAI 字段，同时出现时优先于 max_tokens
	if source.MaxCompletionTokens != nil {
		params.MaxTokens = *source.MaxCompletionTokens
	} else if source.MaxTokens != nil {
		params.MaxTokens = *source.MaxTokens
	}
	if params.MaxTokens <= 0 {
		return UpstreamRequest{}, invalidRequest("max_tokens must be greater than 0 for anthropic upstream")
	}
	if source.Temperature != nil {
		if *source.Temperature < 0 || *source.Temperature > 1 {
			return UpstreamRequest{}, invalidRequest("temperature must be between 0 and 1 for anthropic upstream")
		}
		params.Temperature = anthropic.Float(*source.Temperature)
	}
	if source.TopP != nil {
		if *source.TopP < 0 || *source.TopP > 1 {
			return UpstreamRequest{}, invalidRequest("top_p must be between 0 and 1 for anthropic upstream")
		}
		params.TopP = anthropic.Float(*source.TopP)
	}
	params.StopSequences, err = stopSequences(source.Stop)
	if err != nil {
		return UpstreamRequest{}, err
	}

	converted, err := json.Marshal(params)
	if err != nil {
		return UpstreamRequest{}, fmt.Errorf("marshal anthropic request: %w", err)
	}
	streaming := source.Stream != nil && *source.Stream
	if streaming {
		converted, err = sjson.SetBytes(converted, "stream", true)
		if err != nil {
			return UpstreamRequest{}, fmt.Errorf("enable anthropic streaming: %w", err)
		}
	}
	return UpstreamRequest{Body: converted, BodyChanged: true}, nil
}

func anthropicMessages(source []openAIMessage) ([]anthropic.MessageParam, []anthropic.TextBlockParam, error) {
	messages := make([]anthropic.MessageParam, 0, len(source))
	var system []anthropic.TextBlockParam
	for _, message := range source {
		text, err := messageText(message.Content)
		if err != nil {
			return nil, nil, err
		}
		switch message.Role {
		case "system", "developer":
			// Anthropic 把系统指令放在顶层，不能作为普通 message 发送
			system = append(system, anthropic.TextBlockParam{Text: text})
		case "user":
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
		case "assistant":
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(text)))
		default:
			return nil, nil, invalidRequest("message role is not supported by the anthropic upstream")
		}
	}
	if len(messages) == 0 {
		return nil, nil, invalidRequest("messages must contain a user or assistant message")
	}
	return messages, system, nil
}

func messageText(content json.RawMessage) (string, error) {
	// OpenAI 同时允许纯字符串和 content block 数组，MVP 只保留两者共有的文本能力
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return text, nil
	}

	var blocks []openAIContentBlock
	if err := json.Unmarshal(content, &blocks); err != nil || len(blocks) == 0 {
		return "", invalidRequest("message content must be text")
	}
	var result strings.Builder
	for _, block := range blocks {
		if block.Type != "text" {
			return "", invalidRequest("only text message content is supported by the anthropic upstream")
		}
		result.WriteString(block.Text)
	}
	return result.String(), nil
}

func stopSequences(value json.RawMessage) ([]string, error) {
	if !hasJSONValue(value) {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(value, &single); err == nil {
		return []string{single}, nil
	}
	var multiple []string
	if err := json.Unmarshal(value, &multiple); err != nil {
		return nil, invalidRequest("stop must be a string or an array of strings")
	}
	return multiple, nil
}

func hasJSONValue(value json.RawMessage) bool {
	// 缺失、null 和空数组都表示调用方没有启用该可选能力
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) && !bytes.Equal(trimmed, []byte("[]"))
}
