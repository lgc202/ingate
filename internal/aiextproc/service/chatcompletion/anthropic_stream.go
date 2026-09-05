package chatcompletion

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/tidwall/gjson"

	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

type openAIStreamResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
	Usage   *openAIUsage         `json:"usage,omitempty"`
}

type openAIStreamChoice struct {
	Index        int               `json:"index"`
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason *string           `json:"finish_reason,omitempty"`
}

type openAIStreamDelta struct {
	Role    string  `json:"role,omitempty"`
	Content *string `json:"content,omitempty"`
}

// AnthropicStream 把 Anthropic SSE 增量转换为 OpenAI Chat Completions SSE
// 状态只属于一次请求，负责跨 ExtProc Body chunk 拼接不完整事件。
type AnthropicStream struct {
	buffer              []byte
	clientModel         string
	messageID           string
	created             int64
	started             bool
	roleSent            bool
	finished            bool
	stopReason          anthropic.StopReason
	metadata            ResponseMetadata
	inputTokens         int64
	cacheReadTokens     int64
	cacheCreationTokens int64
	outputTokens        int64
}

// NewAnthropicStream 创建一条 Anthropic 响应流的转换状态。
func NewAnthropicStream(clientModel string) *AnthropicStream {
	return &AnthropicStream{clientModel: clientModel}
}

// Convert 接收任意边界的响应 chunk，并只输出已经完整解析的 OpenAI SSE 事件。
func (s *AnthropicStream) Convert(chunk []byte, endOfStream bool) ([]byte, ResponseMetadata, bool, error) {
	// gRPC chunk 与 SSE 事件没有边界对应关系，先拼入 buffer 再逐个提取完整事件
	s.buffer = append(s.buffer, chunk...)
	var converted []byte
	changed := false
	for {
		event, remaining, found := nextSSEEvent(s.buffer)
		if !found {
			break
		}
		s.buffer = remaining
		output, eventChanged, err := s.convertEvent(event)
		if err != nil {
			return nil, ResponseMetadata{}, false, err
		}
		converted = append(converted, output...)
		changed = eventChanged || changed
	}
	if len(s.buffer) > maxPendingSSEBytes {
		return nil, ResponseMetadata{}, false, errors.New("anthropic stream event exceeds the size limit")
	}
	if endOfStream && len(bytes.TrimSpace(s.buffer)) > 0 {
		// 部分服务结束最后一个 SSE 事件时不带空行，流结束仍要尝试解析尾部数据
		output, eventChanged, err := s.convertEvent(s.buffer)
		if err != nil {
			return nil, ResponseMetadata{}, false, err
		}
		converted = append(converted, output...)
		changed = eventChanged || changed
		s.buffer = nil
	}
	if endOfStream && !s.finished {
		// 上游缺少 message_stop 时仍生成标准结束 chunk 和 [DONE]
		output, eventChanged, err := s.finish()
		if err != nil {
			return nil, ResponseMetadata{}, false, err
		}
		converted = append(converted, output...)
		changed = eventChanged || changed
	}
	return converted, s.metadata, changed, nil
}

func nextSSEEvent(buffer []byte) (event, remaining []byte, found bool) {
	lineFeedIndex := bytes.Index(buffer, []byte("\n\n"))
	carriageReturnIndex := bytes.Index(buffer, []byte("\r\n\r\n"))
	switch {
	case lineFeedIndex >= 0 && (carriageReturnIndex < 0 || lineFeedIndex < carriageReturnIndex):
		return buffer[:lineFeedIndex], buffer[lineFeedIndex+2:], true
	case carriageReturnIndex >= 0:
		return buffer[:carriageReturnIndex], buffer[carriageReturnIndex+4:], true
	default:
		return nil, buffer, false
	}
}

func parseSSEEvent(event []byte) (string, []byte) {
	// 同时兼容 LF 和 CRLF，并忽略 comment、id 等当前转换不需要的 SSE 字段。
	event = bytes.ReplaceAll(event, []byte("\r\n"), []byte("\n"))
	var eventType string
	var data []byte
	for line := range bytes.SplitSeq(event, []byte{'\n'}) {
		if value, ok := bytes.CutPrefix(line, []byte("event:")); ok {
			eventType = string(bytes.TrimSpace(value))
			continue
		}
		if value, ok := bytes.CutPrefix(line, []byte("data:")); ok {
			value = bytes.TrimPrefix(value, []byte{' '})
			data = append(data, value...)
			data = append(data, '\n')
		}
	}
	return eventType, bytes.TrimSuffix(data, []byte{'\n'})
}

func (s *AnthropicStream) convertEvent(event []byte) ([]byte, bool, error) {
	eventType, data := parseSSEEvent(event)
	if len(data) == 0 {
		return nil, false, nil
	}
	eventType = cmp.Or(eventType, gjson.GetBytes(data, "type").String())
	if s.finished {
		return nil, false, errors.New("anthropic stream contains an event after completion")
	}

	switch eventType {
	case "message_start":
		if s.started {
			return nil, false, errors.New("anthropic stream contains multiple message_start events")
		}
		// message_start 建立后续增量事件共用的响应 ID、模型和初始用量
		var start anthropic.MessageStartEvent
		if err := json.Unmarshal(data, &start); err != nil {
			return nil, false, fmt.Errorf("unmarshal anthropic message_start: %w", err)
		}
		if start.Message.ID == "" || !routeconfig.IsValidModelName(start.Message.Model) {
			return nil, false, errors.New("anthropic message_start is missing a valid message ID or model")
		}
		s.messageID = start.Message.ID
		s.created = time.Now().Unix()
		s.started = true
		s.inputTokens = start.Message.Usage.InputTokens
		s.cacheReadTokens = start.Message.Usage.CacheReadInputTokens
		s.cacheCreationTokens = start.Message.Usage.CacheCreationInputTokens
		s.outputTokens = start.Message.Usage.OutputTokens
		if err := s.updateMetadata(start.Message.Model, ""); err != nil {
			return nil, false, err
		}
		return nil, true, nil

	case "content_block_delta":
		if !s.started {
			return nil, false, errors.New("anthropic content_block_delta preceded message_start")
		}
		// 一个 Anthropic 文本 delta 对应一个 OpenAI chat.completion.chunk
		var delta anthropic.ContentBlockDeltaEvent
		if err := json.Unmarshal(data, &delta); err != nil {
			return nil, false, fmt.Errorf("unmarshal anthropic content_block_delta: %w", err)
		}
		if delta.Delta.Type != "text_delta" {
			return nil, false, fmt.Errorf(
				"anthropic stream contains unsupported content delta %q",
				delta.Delta.Type,
			)
		}
		role := ""
		if !s.roleSent {
			role = "assistant"
			s.roleSent = true
		}
		return s.textChunk(role, delta.Delta.Text)

	case "content_block_start":
		if !s.started {
			return nil, false, errors.New("anthropic content_block_start preceded message_start")
		}
		var start anthropic.ContentBlockStartEvent
		if err := json.Unmarshal(data, &start); err != nil {
			return nil, false, fmt.Errorf("unmarshal anthropic content_block_start: %w", err)
		}
		if start.ContentBlock.Type != "text" {
			return nil, false, fmt.Errorf(
				"anthropic stream contains unsupported content block %q",
				start.ContentBlock.Type,
			)
		}
		if start.ContentBlock.Text == "" {
			return nil, false, nil
		}
		role := ""
		if !s.roleSent {
			role = "assistant"
			s.roleSent = true
		}
		return s.textChunk(role, start.ContentBlock.Text)

	case "message_delta":
		if !s.started {
			return nil, false, errors.New("anthropic message_delta preceded message_start")
		}
		var delta anthropic.MessageDeltaEvent
		if err := json.Unmarshal(data, &delta); err != nil {
			return nil, false, fmt.Errorf("unmarshal anthropic message_delta: %w", err)
		}
		if delta.Delta.StopReason != "" {
			s.stopReason = delta.Delta.StopReason
		}
		// Anthropic 流中的 usage 是累计值，后到事件覆盖前值，不能逐次相加
		// 字段未出现时保留 message_start 的值，避免可选字段的零值误覆盖
		if delta.Usage.JSON.InputTokens.Valid() {
			s.inputTokens = delta.Usage.InputTokens
		}
		if delta.Usage.JSON.CacheReadInputTokens.Valid() {
			s.cacheReadTokens = delta.Usage.CacheReadInputTokens
		}
		if delta.Usage.JSON.CacheCreationInputTokens.Valid() {
			s.cacheCreationTokens = delta.Usage.CacheCreationInputTokens
		}
		if delta.Usage.JSON.OutputTokens.Valid() {
			s.outputTokens = delta.Usage.OutputTokens
		}
		finishReason := ""
		if s.stopReason != "" {
			finishReason = openAIFinishReason(s.stopReason)
		}
		if err := s.updateMetadata("", finishReason); err != nil {
			return nil, false, err
		}
		return nil, true, nil

	case "message_stop":
		// finish 统一生成结束原因、可选 usage chunk 和 [DONE]
		return s.finish()

	case "error":
		// 流中错误也转换为 OpenAI 错误对象，并正常结束 SSE，避免客户端一直等待
		converted, changed, err := RewriteAnthropicErrorResponse(data)
		if err != nil {
			return nil, false, err
		}
		if !changed {
			return nil, false, nil
		}
		output := appendSSEData(nil, converted)
		output = appendSSEData(output, []byte("[DONE]"))
		s.finished = true
		return output, false, nil

	case "ping", "content_block_stop":
		return nil, false, nil

	default:
		return nil, false, nil
	}
}

func (s *AnthropicStream) textChunk(role, content string) ([]byte, bool, error) {
	chunk := openAIStreamResponse{
		ID:      s.messageID,
		Object:  "chat.completion.chunk",
		Created: s.created,
		Model:   s.clientModel,
		Choices: []openAIStreamChoice{{
			Index: 0,
			Delta: openAIStreamDelta{
				Role:    role,
				Content: &content,
			},
		}},
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return nil, false, fmt.Errorf("marshal OpenAI stream chunk: %w", err)
	}
	return appendSSEData(nil, data), false, nil
}

func (s *AnthropicStream) finish() ([]byte, bool, error) {
	if s.finished {
		return nil, false, nil
	}
	if !s.started {
		return nil, false, errors.New("anthropic stream ended before message_start")
	}
	s.stopReason = cmp.Or(s.stopReason, anthropic.StopReasonEndTurn)
	finishReason := openAIFinishReason(s.stopReason)
	if err := s.updateMetadata("", finishReason); err != nil {
		return nil, false, err
	}
	s.metadata.Usage.Final = s.metadata.Usage.Found

	finishChunk := openAIStreamResponse{
		ID:      s.messageID,
		Object:  "chat.completion.chunk",
		Created: s.created,
		Model:   s.clientModel,
		Choices: []openAIStreamChoice{{
			Index:        0,
			Delta:        openAIStreamDelta{},
			FinishReason: &finishReason,
		}},
	}
	finishData, err := json.Marshal(finishChunk)
	if err != nil {
		return nil, false, fmt.Errorf("marshal OpenAI finish chunk: %w", err)
	}
	output := appendSSEData(nil, finishData)

	if s.metadata.Usage.Found {
		// 单独的空 choices chunk 对齐 OpenAI include_usage 流式响应约定
		usage := s.metadata.Usage
		usageData, err := json.Marshal(openAIStreamResponse{
			ID:      s.messageID,
			Object:  "chat.completion.chunk",
			Created: s.created,
			Model:   s.clientModel,
			Choices: []openAIStreamChoice{},
			Usage: &openAIUsage{
				PromptTokens:     usage.InputTokens,
				CompletionTokens: usage.OutputTokens,
				TotalTokens:      usage.TotalTokens,
			},
		})
		if err != nil {
			return nil, false, fmt.Errorf("marshal OpenAI stream usage: %w", err)
		}
		output = appendSSEData(output, usageData)
	}
	output = appendSSEData(output, []byte("[DONE]"))
	s.finished = true
	return output, true, nil
}

func (s *AnthropicStream) updateMetadata(model, finishReason string) error {
	usage := anthropicUsage(s.inputTokens, s.outputTokens, s.cacheReadTokens, s.cacheCreationTokens)
	if !usage.Found {
		return errors.New("anthropic stream contains invalid token usage")
	}
	if model != "" {
		s.metadata.ResponseModel = model
	}
	if finishReason != "" {
		s.metadata.FinishReason = finishReason
	}
	if usage.Found {
		s.metadata.Usage = usage
	}
	return nil
}

func appendSSEData(target, data []byte) []byte {
	target = append(target, "data: "...)
	target = append(target, data...)
	return append(target, '\n', '\n')
}
