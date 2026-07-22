// Package usage 提取 AI Proxy 归一化响应中的最终 Token 用量
package usage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/pkg/llm/sse"
)

type tokenUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// Stream 增量提取 OpenAI-compatible SSE 中最后一个合法 usage
type Stream struct {
	decoder sse.Decoder
	tokens  int64
	found   bool
	done    bool
}

// ParseJSON 提取 OpenAI-compatible 普通响应中的 usage.total_tokens
func ParseJSON(body []byte) (int64, bool, error) {
	var response struct {
		Usage *tokenUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, false, fmt.Errorf("decode token usage response: %w", err)
	}
	if response.Usage == nil {
		return 0, false, nil
	}
	tokens, err := totalTokens(*response.Usage)
	if err != nil {
		return 0, false, err
	}
	return tokens, true, nil
}

// Push 写入一个任意边界的 SSE 网络分块
func (s *Stream) Push(chunk []byte) error {
	events, err := s.decoder.Push(chunk)
	if err != nil {
		return fmt.Errorf("parse token usage SSE: %w", err)
	}
	return s.consume(events)
}

// Finish 完成 SSE 解析并处理未以空行结束的最后一个事件
func (s *Stream) Finish() error {
	events, err := s.decoder.Finish()
	if err != nil {
		return fmt.Errorf("finish token usage SSE: %w", err)
	}
	return s.consume(events)
}

// TotalTokens 返回流中最后一个合法 usage.total_tokens
func (s *Stream) TotalTokens() (int64, bool) {
	return s.tokens, s.found
}

// Complete 返回流是否已经收到 OpenAI-compatible 完成标记
func (s *Stream) Complete() bool {
	return s.done
}

func (s *Stream) consume(events []sse.Event) error {
	for _, event := range events {
		data := bytes.TrimSpace(event.Data)
		if len(data) == 0 {
			continue
		}
		if s.done {
			return errors.New("token usage SSE contains data after [DONE]")
		}
		if bytes.Equal(data, []byte("[DONE]")) {
			s.done = true
			continue
		}
		var chunk struct {
			Usage *tokenUsage     `json:"usage"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(data, &chunk); err != nil {
			return fmt.Errorf("decode token usage SSE event: %w", err)
		}
		if len(chunk.Error) > 0 || chunk.Usage == nil {
			continue
		}
		tokens, err := totalTokens(*chunk.Usage)
		if err != nil {
			return fmt.Errorf("validate token usage SSE event: %w", err)
		}
		s.tokens = tokens
		s.found = true
	}
	return nil
}

func totalTokens(value tokenUsage) (int64, error) {
	if value.PromptTokens < 0 || value.CompletionTokens < 0 || value.TotalTokens < 0 {
		return 0, errors.New("token usage contains a negative value")
	}
	if value.TotalTokens > 0 || value.PromptTokens == 0 && value.CompletionTokens == 0 {
		return value.TotalTokens, nil
	}
	if value.PromptTokens > int64(^uint64(0)>>1)-value.CompletionTokens {
		return 0, errors.New("token usage total overflows int64")
	}
	return value.PromptTokens + value.CompletionTokens, nil
}
