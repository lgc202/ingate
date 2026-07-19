package gemini

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/openai"
)

type requestBody struct {
	Contents          []content         `json:"contents"`
	SystemInstruction *content          `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text         *string         `json:"text,omitempty"`
	InlineData   json.RawMessage `json:"inlineData,omitempty"`
	FunctionCall json.RawMessage `json:"functionCall,omitempty"`
}

type generationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type response struct {
	Candidates    []candidate    `json:"candidates"`
	UsageMetadata usageMetadata  `json:"usageMetadata"`
	ModelVersion  string         `json:"modelVersion"`
	ResponseID    string         `json:"responseId"`
	Error         *upstreamError `json:"error"`
}

type candidate struct {
	Content      content `json:"content"`
	FinishReason string  `json:"finishReason"`
	Index        int     `json:"index"`
}

type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type upstreamError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// EndpointPath 返回相对于 Gemini API Base Path 的模型请求路径
func EndpointPath(upstreamModel string, stream bool) (string, error) {
	if strings.TrimSpace(upstreamModel) == "" {
		return "", fmt.Errorf("%w: upstream model must not be empty", llm.ErrInvalidRequest)
	}
	action := "generateContent"
	if stream {
		action = "streamGenerateContent?alt=sse"
	}
	return "/models/" + url.PathEscape(upstreamModel) + ":" + action, nil
}

// TransformRequest 把 openai.DecodeRequest 返回的文本请求转换为 Gemini generateContent 请求
func TransformRequest(request openai.Request) ([]byte, error) {
	transformed := requestBody{Contents: make([]content, 0, len(request.Messages))}
	var system []string
	for _, item := range request.Messages {
		if item.Role == openai.RoleSystem {
			system = append(system, item.Content)
			continue
		}
		role := "user"
		if item.Role == openai.RoleAssistant {
			role = "model"
		}
		text := item.Content
		transformed.Contents = append(transformed.Contents, content{
			Role:  role,
			Parts: []part{{Text: &text}},
		})
	}
	if len(transformed.Contents) == 0 {
		return nil, fmt.Errorf("%w: Gemini requires at least one user or assistant message", llm.ErrInvalidRequest)
	}
	if len(system) > 0 {
		text := strings.Join(system, "\n\n")
		transformed.SystemInstruction = &content{Parts: []part{{Text: &text}}}
	}
	if request.Temperature != nil || request.TopP != nil || request.MaxTokens != nil || len(request.Stop) > 0 {
		transformed.GenerationConfig = &generationConfig{
			Temperature:     request.Temperature,
			TopP:            request.TopP,
			MaxOutputTokens: request.MaxTokens,
			StopSequences:   request.Stop,
		}
	}

	encoded, err := json.Marshal(transformed)
	if err != nil {
		return nil, fmt.Errorf("encode Gemini request: %w", err)
	}
	return encoded, nil
}

// TransformResponse 把 Gemini 成功响应转换为 OpenAI-compatible 响应
func TransformResponse(body []byte, publicModel string) ([]byte, error) {
	if strings.TrimSpace(publicModel) == "" {
		return nil, fmt.Errorf("%w: public model must not be empty", llm.ErrInvalidResponse)
	}
	var original response
	if err := json.Unmarshal(body, &original); err != nil {
		return nil, fmt.Errorf("%w: decode Gemini response: %v", llm.ErrInvalidResponse, err)
	}
	if original.Error != nil {
		return TransformError(body, original.Error.Code), nil
	}
	if len(original.Candidates) == 0 {
		return nil, fmt.Errorf("%w: Gemini response has no candidates", llm.ErrInvalidResponse)
	}

	choices := make([]openai.CompletionChoice, 0, len(original.Candidates))
	for _, item := range original.Candidates {
		text, err := textContent(item.Content)
		if err != nil {
			return nil, err
		}
		choices = append(choices, openai.CompletionChoice{
			Index: item.Index,
			Message: openai.ResponseMessage{
				Role:    openai.RoleAssistant,
				Content: text,
			},
			FinishReason: mapFinishReason(item.FinishReason),
		})
	}

	id := original.ResponseID
	if id == "" {
		id = "chatcmpl-gemini"
	}
	transformed := openai.ChatCompletion{
		ID:      id,
		Object:  openai.ObjectChatCompletion,
		Created: time.Now().Unix(),
		Model:   publicModel,
		Choices: choices,
		Usage:   mapUsage(original.UsageMetadata),
	}
	encoded, err := json.Marshal(transformed)
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI response: %w", err)
	}
	return encoded, nil
}

// TransformError 把 Gemini 错误转换为 OpenAI-compatible 错误信封
func TransformError(body []byte, statusCode int) []byte {
	var envelope struct {
		Error upstreamError `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error.Message == "" {
		return openai.EncodeError(openai.DefaultError(statusCode, "upstream returned an invalid Gemini error response"))
	}
	if statusCode == 0 {
		statusCode = envelope.Error.Code
	}
	detail := openai.ErrorDetail{
		Message: envelope.Error.Message,
		Type:    openai.DefaultError(statusCode, "").Type,
		Code:    envelope.Error.Status,
	}
	if detail.Code == "" && envelope.Error.Code != 0 {
		detail.Code = fmt.Sprintf("%d", envelope.Error.Code)
	}
	return openai.EncodeError(detail)
}

func textContent(value content) (string, error) {
	var text strings.Builder
	for _, item := range value.Parts {
		if len(item.InlineData) > 0 || len(item.FunctionCall) > 0 {
			return "", fmt.Errorf("%w: Gemini returned a non-text content part", llm.ErrUnsupportedFeature)
		}
		if item.Text == nil {
			return "", fmt.Errorf("%w: Gemini returned an empty content part", llm.ErrInvalidResponse)
		}
		text.WriteString(*item.Text)
	}
	return text.String(), nil
}

func mapUsage(value usageMetadata) *openai.Usage {
	total := value.TotalTokenCount
	if total == 0 {
		total = value.PromptTokenCount + value.CandidatesTokenCount
	}
	return &openai.Usage{
		PromptTokens:     value.PromptTokenCount,
		CompletionTokens: value.CandidatesTokenCount,
		TotalTokens:      total,
	}
}

func mapFinishReason(reason string) *openai.FinishReason {
	if reason == "" || reason == "FINISH_REASON_UNSPECIFIED" {
		return nil
	}
	mapped := openai.FinishReasonContentFilter
	switch reason {
	case "STOP":
		mapped = openai.FinishReasonStop
	case "MAX_TOKENS":
		mapped = openai.FinishReasonLength
	}
	return &mapped
}
