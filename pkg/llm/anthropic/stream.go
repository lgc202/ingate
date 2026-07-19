package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/sse"
)

type streamEvent struct {
	Type         string         `json:"type"`
	Message      *response      `json:"message"`
	Index        int            `json:"index"`
	ContentBlock *contentBlock  `json:"content_block"`
	Delta        *streamDelta   `json:"delta"`
	Usage        *usage         `json:"usage"`
	Error        *upstreamError `json:"error"`
}

type streamDelta struct {
	Type       string  `json:"type"`
	Text       string  `json:"text"`
	StopReason *string `json:"stop_reason"`
}

// Stream 增量把 Anthropic Messages SSE 转换为 OpenAI-compatible SSE
type Stream struct {
	decoder          sse.Decoder
	publicModel      string
	id               string
	created          int64
	promptTokens     int
	completionTokens int
	activeBlocks     map[int]bool
	started          bool
	finished         bool
	done             bool
}

// NewStream 创建 Anthropic SSE 转换器
func NewStream(publicModel string) (*Stream, error) {
	if strings.TrimSpace(publicModel) == "" {
		return nil, fmt.Errorf("%w: public model must not be empty", llm.ErrInvalidStream)
	}
	return &Stream{
		publicModel:  publicModel,
		created:      time.Now().Unix(),
		activeBlocks: make(map[int]bool),
	}, nil
}

// Push 转换一个任意大小和边界的上游网络分块
func (s *Stream) Push(chunk []byte) ([]byte, error) {
	events, err := s.decoder.Push(chunk)
	if err != nil {
		return nil, fmt.Errorf("%w: parse Anthropic SSE: %v", llm.ErrInvalidStream, err)
	}
	return s.transform(events)
}

// Finish 完成 Anthropic SSE 转换，并校验 message_stop 已到达
func (s *Stream) Finish() ([]byte, error) {
	events, err := s.decoder.Finish()
	if err != nil {
		return nil, fmt.Errorf("%w: finish Anthropic SSE: %v", llm.ErrInvalidStream, err)
	}
	output, err := s.transform(events)
	if err != nil {
		return nil, err
	}
	if !s.done {
		return nil, fmt.Errorf("%w: Anthropic stream ended before message_stop", llm.ErrInvalidStream)
	}
	return output, nil
}

func (s *Stream) transform(events []sse.Event) ([]byte, error) {
	var output []byte
	for _, event := range events {
		data := bytes.TrimSpace(event.Data)
		if len(data) == 0 {
			continue
		}
		if s.done {
			return nil, fmt.Errorf("%w: received Anthropic data after message_stop", llm.ErrInvalidStream)
		}

		var original streamEvent
		if err := json.Unmarshal(data, &original); err != nil {
			return nil, fmt.Errorf("%w: decode Anthropic SSE data: %v", llm.ErrInvalidStream, err)
		}
		switch original.Type {
		case "ping":
			continue
		case "message_start":
			if s.started {
				return nil, fmt.Errorf("%w: Anthropic message_start was repeated", llm.ErrInvalidStream)
			}
			if original.Message == nil || original.Message.ID == "" {
				return nil, fmt.Errorf("%w: Anthropic message_start is missing message", llm.ErrInvalidStream)
			}
			s.id = original.Message.ID
			s.promptTokens = original.Message.Usage.InputTokens
			s.completionTokens = original.Message.Usage.OutputTokens
			s.started = true
			empty := ""
			chunk := s.chunk([]llm.ChunkChoice{{
				Index: 0,
				Delta: llm.MessageDelta{Role: llm.RoleAssistant, Content: &empty},
			}})
			output = appendJSONEvent(output, chunk)
		case "content_block_start":
			if !s.started || s.finished {
				return nil, fmt.Errorf("%w: Anthropic content_block_start is out of order", llm.ErrInvalidStream)
			}
			if original.ContentBlock == nil {
				return nil, fmt.Errorf("%w: Anthropic content_block_start is missing content_block", llm.ErrInvalidStream)
			}
			if original.ContentBlock.Type != "text" || original.ContentBlock.Text == nil {
				return nil, fmt.Errorf("%w: Anthropic content block type %q is not supported",
					llm.ErrUnsupportedFeature, original.ContentBlock.Type)
			}
			if s.activeBlocks[original.Index] {
				return nil, fmt.Errorf("%w: Anthropic content block %d was started twice", llm.ErrInvalidStream, original.Index)
			}
			s.activeBlocks[original.Index] = true
			if *original.ContentBlock.Text != "" {
				text := *original.ContentBlock.Text
				output = appendJSONEvent(output, s.chunk([]llm.ChunkChoice{{
					Index: 0,
					Delta: llm.MessageDelta{Content: &text},
				}}))
			}
		case "content_block_delta":
			if !s.activeBlocks[original.Index] {
				return nil, fmt.Errorf("%w: Anthropic content block %d is not active", llm.ErrInvalidStream, original.Index)
			}
			if original.Delta == nil {
				return nil, fmt.Errorf("%w: Anthropic content_block_delta is missing delta", llm.ErrInvalidStream)
			}
			if original.Delta.Type != "text_delta" {
				return nil, fmt.Errorf("%w: Anthropic delta type %q is not supported",
					llm.ErrUnsupportedFeature, original.Delta.Type)
			}
			text := original.Delta.Text
			output = appendJSONEvent(output, s.chunk([]llm.ChunkChoice{{
				Index: 0,
				Delta: llm.MessageDelta{Content: &text},
			}}))
		case "content_block_stop":
			if !s.activeBlocks[original.Index] {
				return nil, fmt.Errorf("%w: Anthropic content block %d was not active", llm.ErrInvalidStream, original.Index)
			}
			delete(s.activeBlocks, original.Index)
		case "message_delta":
			if !s.started {
				return nil, fmt.Errorf("%w: Anthropic message_delta arrived before message_start", llm.ErrInvalidStream)
			}
			if original.Delta == nil {
				return nil, fmt.Errorf("%w: Anthropic message_delta is missing delta", llm.ErrInvalidStream)
			}
			if len(s.activeBlocks) > 0 {
				return nil, fmt.Errorf("%w: Anthropic message_delta arrived before content blocks stopped", llm.ErrInvalidStream)
			}
			if s.finished {
				return nil, fmt.Errorf("%w: Anthropic message_delta was repeated", llm.ErrInvalidStream)
			}
			if original.Usage != nil {
				s.completionTokens = original.Usage.OutputTokens
			}
			output = appendJSONEvent(output, s.chunk([]llm.ChunkChoice{{
				Index:        0,
				Delta:        llm.MessageDelta{},
				FinishReason: mapFinishReason(original.Delta.StopReason),
			}}))
			s.finished = true
		case "message_stop":
			if !s.started || !s.finished {
				return nil, fmt.Errorf("%w: Anthropic message_stop arrived before message_delta", llm.ErrInvalidStream)
			}
			usage := &llm.Usage{
				PromptTokens:     s.promptTokens,
				CompletionTokens: s.completionTokens,
				TotalTokens:      s.promptTokens + s.completionTokens,
			}
			chunk := s.chunk(nil)
			chunk.Choices = []llm.ChunkChoice{}
			chunk.Usage = usage
			output = appendJSONEvent(output, chunk)
			output = append(output, sse.EncodeData([]byte("[DONE]"))...)
			s.done = true
		case "error":
			if original.Error == nil {
				return nil, fmt.Errorf("%w: Anthropic error event is missing error details", llm.ErrInvalidStream)
			}
			detail := llm.APIError{Message: original.Error.Message, Type: original.Error.Type}
			if detail.Type == "" {
				detail.Type = llm.DefaultAPIError(502, "").Type
			}
			output = append(output, sse.EncodeData(llm.EncodeError(detail))...)
			output = append(output, sse.EncodeData([]byte("[DONE]"))...)
			s.done = true
		default:
			return nil, fmt.Errorf("%w: unknown Anthropic event type %q", llm.ErrInvalidStream, original.Type)
		}
	}
	return output, nil
}

func (s *Stream) chunk(choices []llm.ChunkChoice) llm.ChatCompletionChunk {
	return llm.ChatCompletionChunk{
		ID:      s.id,
		Object:  llm.ObjectChatCompletionChunk,
		Created: s.created,
		Model:   s.publicModel,
		Choices: choices,
	}
}

func appendJSONEvent(output []byte, value any) []byte {
	// 调用点只传入不含 channel、函数或循环引用的协议结构体，编码不会失败
	data, _ := json.Marshal(value)
	return append(output, sse.EncodeData(data)...)
}
