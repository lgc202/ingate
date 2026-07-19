package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lgc202/ingate/pkg/llm"
)

const (
	// MessagesPath 是相对于 Anthropic API Base Path 的请求路径
	MessagesPath = "/messages"
	// DefaultMaxTokens 是调用方省略 max_tokens 时使用的 Anthropic 必填值
	DefaultMaxTokens = 4096
)

type request struct {
	Model         string    `json:"model"`
	Messages      []message `json:"messages"`
	System        string    `json:"system,omitempty"`
	MaxTokens     int       `json:"max_tokens"`
	StopSequences []string  `json:"stop_sequences,omitempty"`
	Stream        bool      `json:"stream,omitempty"`
	Temperature   *float64  `json:"temperature,omitempty"`
	TopP          *float64  `json:"top_p,omitempty"`
}

type message struct {
	Role    llm.Role `json:"role"`
	Content string   `json:"content"`
}

type response struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []contentBlock `json:"content"`
	StopReason *string        `json:"stop_reason"`
	Usage      usage          `json:"usage"`
	Error      *upstreamError `json:"error"`
}

type contentBlock struct {
	Type string  `json:"type"`
	Text *string `json:"text"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type upstreamError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// TransformRequest 把受支持的 OpenAI-compatible 文本请求转换为 Anthropic Messages 请求
func TransformRequest(body []byte, upstreamModel string) ([]byte, error) {
	if strings.TrimSpace(upstreamModel) == "" {
		return nil, fmt.Errorf("%w: upstream model must not be empty", llm.ErrInvalidRequest)
	}
	original, err := llm.DecodeChatRequest(body)
	if err != nil {
		return nil, err
	}
	if err := original.ValidateSupported(); err != nil {
		return nil, err
	}

	transformed := request{
		Model:         upstreamModel,
		Messages:      make([]message, 0, len(original.Messages)),
		MaxTokens:     DefaultMaxTokens,
		StopSequences: original.Stop,
		Stream:        original.Streaming(),
		Temperature:   original.Temperature,
		TopP:          original.TopP,
	}
	if original.MaxTokens != nil {
		transformed.MaxTokens = *original.MaxTokens
	}

	var system []string
	for _, item := range original.Messages {
		if item.Role == llm.RoleSystem {
			system = append(system, item.Content)
			continue
		}
		transformed.Messages = append(transformed.Messages, message{Role: item.Role, Content: item.Content})
	}
	if len(transformed.Messages) == 0 {
		return nil, fmt.Errorf("%w: Anthropic requires at least one user or assistant message", llm.ErrInvalidRequest)
	}
	transformed.System = strings.Join(system, "\n\n")

	encoded, err := json.Marshal(transformed)
	if err != nil {
		return nil, fmt.Errorf("encode Anthropic request: %w", err)
	}
	return encoded, nil
}

// TransformResponse 把 Anthropic Messages 成功响应转换为 OpenAI-compatible 响应
func TransformResponse(body []byte, publicModel string) ([]byte, error) {
	if strings.TrimSpace(publicModel) == "" {
		return nil, fmt.Errorf("%w: public model must not be empty", llm.ErrInvalidResponse)
	}
	var original response
	if err := json.Unmarshal(body, &original); err != nil {
		return nil, fmt.Errorf("%w: decode Anthropic response: %v", llm.ErrInvalidResponse, err)
	}
	if original.Error != nil {
		return TransformError(body, 502), nil
	}
	if original.ID == "" || original.Type != "message" || original.Role != "assistant" {
		return nil, fmt.Errorf("%w: Anthropic response is missing message metadata", llm.ErrInvalidResponse)
	}

	var content strings.Builder
	for i, block := range original.Content {
		if block.Type != "text" || block.Text == nil {
			return nil, fmt.Errorf("%w: Anthropic content[%d] type %q is not supported",
				llm.ErrUnsupportedFeature, i, block.Type)
		}
		content.WriteString(*block.Text)
	}

	finishReason := mapFinishReason(original.StopReason)
	transformed := llm.ChatCompletion{
		ID:      original.ID,
		Object:  llm.ObjectChatCompletion,
		Created: time.Now().Unix(),
		Model:   publicModel,
		Choices: []llm.CompletionChoice{{
			Index: 0,
			Message: llm.ResponseMessage{
				Role:    llm.RoleAssistant,
				Content: content.String(),
			},
			FinishReason: finishReason,
		}},
		Usage: &llm.Usage{
			PromptTokens:     original.Usage.InputTokens,
			CompletionTokens: original.Usage.OutputTokens,
			TotalTokens:      original.Usage.InputTokens + original.Usage.OutputTokens,
		},
	}
	encoded, err := json.Marshal(transformed)
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI response: %w", err)
	}
	return encoded, nil
}

// TransformError 把 Anthropic 错误转换为 OpenAI-compatible 错误信封
func TransformError(body []byte, statusCode int) []byte {
	var envelope struct {
		Error upstreamError `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error.Message == "" {
		return llm.EncodeError(llm.DefaultAPIError(statusCode, "upstream returned an invalid Anthropic error response"))
	}
	detail := llm.APIError{
		Message: envelope.Error.Message,
		Type:    envelope.Error.Type,
	}
	if detail.Type == "" {
		detail.Type = llm.DefaultAPIError(statusCode, "").Type
	}
	return llm.EncodeError(detail)
}

func mapFinishReason(reason *string) *llm.FinishReason {
	if reason == nil || *reason == "" {
		return nil
	}
	mapped := llm.FinishReason(*reason)
	switch *reason {
	case "end_turn", "stop_sequence":
		mapped = llm.FinishReasonStop
	case "max_tokens":
		mapped = llm.FinishReasonLength
	}
	return &mapped
}
