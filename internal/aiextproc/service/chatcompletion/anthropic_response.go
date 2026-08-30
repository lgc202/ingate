package chatcompletion

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/tidwall/gjson"
)

type openAIResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []openAIResponseChoice `json:"choices"`
	Usage   openAIUsage            `json:"usage"`
}

type openAIResponseChoice struct {
	Index        int                   `json:"index"`
	Message      openAIResponseMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

type openAIResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIUsage struct {
	PromptTokens     uint64 `json:"prompt_tokens"`
	CompletionTokens uint64 `json:"completion_tokens"`
	TotalTokens      uint64 `json:"total_tokens"`
}

// RewriteAnthropicResponse 把完整的 Anthropic Messages 响应转换为 OpenAI Chat Completions 响应。
func RewriteAnthropicResponse(body []byte, clientModel string) ([]byte, ResponseMetadata, error) {
	convertedError, isError, err := RewriteAnthropicErrorResponse(body)
	if err != nil {
		return nil, ResponseMetadata{}, err
	}
	if isError {
		return convertedError, ResponseMetadata{}, nil
	}

	var source anthropic.Message
	if err := json.Unmarshal(body, &source); err != nil {
		return nil, ResponseMetadata{}, fmt.Errorf("unmarshal anthropic response: %w", err)
	}
	if source.ID == "" || source.Model == "" {
		return nil, ResponseMetadata{}, errors.New("unmarshal anthropic response: missing message ID or model")
	}
	usage := anthropicUsage(source.Usage.InputTokens, source.Usage.OutputTokens,
		source.Usage.CacheReadInputTokens, source.Usage.CacheCreationInputTokens)
	finishReason := openAIFinishReason(source.StopReason)
	var content strings.Builder
	for _, block := range source.Content {
		// 当前请求侧只接受文本，因此响应侧也只输出文本块，避免伪造工具调用语义
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	converted, err := json.Marshal(openAIResponse{
		ID:      source.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   clientModel,
		Choices: []openAIResponseChoice{{
			Index: 0,
			Message: openAIResponseMessage{
				Role:    "assistant",
				Content: content.String(),
			},
			FinishReason: finishReason,
		}},
		Usage: openAIUsage{
			PromptTokens:     usage.InputTokens,
			CompletionTokens: usage.OutputTokens,
			TotalTokens:      usage.TotalTokens,
		},
	})
	if err != nil {
		return nil, ResponseMetadata{}, fmt.Errorf("marshal OpenAI response: %w", err)
	}
	return converted, ResponseMetadata{
		ResponseModel: source.Model,
		FinishReason:  finishReason,
		Usage:         usage,
	}, nil
}

// RewriteAnthropicErrorResponse 只转换能够确认来自 Anthropic 的错误对象
// 中间代理生成的普通 HTTP 错误由调用方依据 changed=false 原样透传。
func RewriteAnthropicErrorResponse(body []byte) (converted []byte, changed bool, err error) {
	if gjson.GetBytes(body, "type").String() != "error" {
		return nil, false, nil
	}
	message := gjson.GetBytes(body, "error.message").String()
	errorType := gjson.GetBytes(body, "error.type").String()
	if message == "" || errorType == "" {
		return nil, false, errors.New("unmarshal anthropic error response: missing error type or message")
	}
	converted, err = json.Marshal(map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    errorType,
			"code":    errorType,
		},
	})
	if err != nil {
		return nil, false, fmt.Errorf("marshal OpenAI error response: %w", err)
	}
	return converted, true, nil
}

func anthropicUsage(input, output, cacheRead, cacheCreation int64) Usage {
	// Anthropic 将缓存读取和缓存写入 Token 从普通输入 Token 中拆开上报
	// Ingate 的统一 input_tokens 表示本次请求计费涉及的全部输入 Token
	input += cacheRead + cacheCreation
	if input < 0 || output < 0 {
		return Usage{}
	}
	return Usage{
		InputTokens:  uint64(input),
		OutputTokens: uint64(output),
		TotalTokens:  uint64(input + output),
		Found:        true,
	}
}

func openAIFinishReason(reason anthropic.StopReason) string {
	// 只映射 OpenAI 客户端能够稳定理解的结束原因，其余正常结束统一为 stop
	switch reason {
	case anthropic.StopReasonMaxTokens:
		return "length"
	case anthropic.StopReasonToolUse:
		return "tool_calls"
	case anthropic.StopReasonRefusal:
		return "content_filter"
	default:
		return "stop"
	}
}
