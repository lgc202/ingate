package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/openai"
)

const (
	// MessagesPath 是相对于 Anthropic API Base Path 的请求路径
	MessagesPath = "/messages"
	// defaultMaxTokens 是调用方省略 max_tokens 时使用的 Anthropic 必填值
	defaultMaxTokens = 4096
)

type requestBody struct {
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
	Role    openai.Role `json:"role"`
	Content string      `json:"content"`
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

// TransformRequest 把 openai.DecodeRequest 返回的文本请求转换为 Anthropic Messages 请求
func TransformRequest(request openai.Request, upstreamModel string) ([]byte, error) {
	if strings.TrimSpace(upstreamModel) == "" {
		return nil, fmt.Errorf("%w: upstream model must not be empty", llm.ErrInvalidRequest)
	}

	transformed := requestBody{
		Model:         upstreamModel,
		Messages:      make([]message, 0, len(request.Messages)),
		MaxTokens:     defaultMaxTokens,
		StopSequences: request.Stop,
		Stream:        request.Streaming(),
		Temperature:   request.Temperature,
		TopP:          request.TopP,
	}
	if request.MaxTokens != nil {
		transformed.MaxTokens = *request.MaxTokens
	}

	var system []string
	for _, item := range request.Messages {
		if item.Role == openai.RoleSystem {
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
	transformed := openai.ChatCompletion{
		ID:      original.ID,
		Object:  openai.ObjectChatCompletion,
		Created: time.Now().Unix(),
		Model:   publicModel,
		Choices: []openai.CompletionChoice{{
			Index: 0,
			Message: openai.ResponseMessage{
				Role:    openai.RoleAssistant,
				Content: content.String(),
			},
			FinishReason: finishReason,
		}},
		Usage: &openai.Usage{
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
		return openai.EncodeError(openai.DefaultError(statusCode, "upstream returned an invalid Anthropic error response"))
	}
	detail := openai.ErrorDetail{
		Message: envelope.Error.Message,
		Type:    envelope.Error.Type,
	}
	if detail.Type == "" {
		detail.Type = openai.DefaultError(statusCode, "").Type
	}
	return openai.EncodeError(detail)
}

func mapFinishReason(reason *string) *openai.FinishReason {
	if reason == nil || *reason == "" {
		return nil
	}
	mapped := openai.FinishReason(*reason)
	switch *reason {
	case "end_turn", "stop_sequence":
		mapped = openai.FinishReasonStop
	case "max_tokens":
		mapped = openai.FinishReasonLength
	}
	return &mapped
}
