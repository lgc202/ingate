package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/openai"
	"github.com/lgc202/ingate/pkg/llm/sse"
)

// Stream 增量把 Gemini SSE 转换为 OpenAI-compatible SSE
type Stream struct {
	decoder          sse.Decoder
	publicModel      string
	id               string
	created          int64
	promptTokens     int
	completionTokens int
	totalTokens      int
	roleSent         map[int]bool
	seenCandidates   map[int]bool
	finished         map[int]bool
	done             bool
}

// NewStream 创建 Gemini SSE 转换器
func NewStream(publicModel string) (*Stream, error) {
	if strings.TrimSpace(publicModel) == "" {
		return nil, fmt.Errorf("%w: public model must not be empty", llm.ErrInvalidStream)
	}
	return &Stream{
		publicModel:    publicModel,
		id:             "chatcmpl-gemini",
		created:        time.Now().Unix(),
		roleSent:       make(map[int]bool),
		seenCandidates: make(map[int]bool),
		finished:       make(map[int]bool),
	}, nil
}

// Push 转换一个任意大小和边界的上游网络分块
func (s *Stream) Push(chunk []byte) ([]byte, error) {
	events, err := s.decoder.Push(chunk)
	if err != nil {
		return nil, fmt.Errorf("%w: parse Gemini SSE: %v", llm.ErrInvalidStream, err)
	}
	return s.transform(events)
}

// Finish 完成 Gemini SSE 转换，在确认收到 finishReason 后输出 [DONE]
func (s *Stream) Finish() ([]byte, error) {
	events, err := s.decoder.Finish()
	if err != nil {
		return nil, fmt.Errorf("%w: finish Gemini SSE: %v", llm.ErrInvalidStream, err)
	}
	output, err := s.transform(events)
	if err != nil {
		return nil, err
	}
	if s.done {
		return output, nil
	}
	if !s.allCandidatesFinished() {
		return nil, fmt.Errorf("%w: Gemini stream ended before finishReason", llm.ErrInvalidStream)
	}
	output = append(output, sse.EncodeData([]byte("[DONE]"))...)
	s.done = true
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
			return nil, fmt.Errorf("%w: received Gemini data after [DONE]", llm.ErrInvalidStream)
		}

		var original response
		if err := json.Unmarshal(data, &original); err != nil {
			return nil, fmt.Errorf("%w: decode Gemini SSE data: %v", llm.ErrInvalidStream, err)
		}
		if original.Error != nil {
			output = append(output, sse.EncodeData(TransformError(data, original.Error.Code))...)
			output = append(output, sse.EncodeData([]byte("[DONE]"))...)
			s.done = true
			continue
		}
		if original.ResponseID != "" {
			s.id = original.ResponseID
		}
		if original.UsageMetadata != (usageMetadata{}) {
			s.promptTokens = original.UsageMetadata.PromptTokenCount
			s.completionTokens = original.UsageMetadata.CandidatesTokenCount
			s.totalTokens = original.UsageMetadata.TotalTokenCount
			if s.totalTokens == 0 {
				s.totalTokens = s.promptTokens + s.completionTokens
			}
		}

		choices := make([]openai.ChunkChoice, 0, len(original.Candidates))
		for _, item := range original.Candidates {
			s.seenCandidates[item.Index] = true
			text, err := textContent(item.Content)
			if err != nil {
				return nil, err
			}
			delta := openai.MessageDelta{}
			if !s.roleSent[item.Index] {
				delta.Role = openai.RoleAssistant
				s.roleSent[item.Index] = true
			}
			if text != "" {
				delta.Content = &text
			}
			finishReason := mapFinishReason(item.FinishReason)
			if finishReason != nil {
				s.finished[item.Index] = true
			}
			choices = append(choices, openai.ChunkChoice{
				Index:        item.Index,
				Delta:        delta,
				FinishReason: finishReason,
			})
		}

		chunk := openai.ChatCompletionChunk{
			ID:      s.id,
			Object:  openai.ObjectChatCompletionChunk,
			Created: s.created,
			Model:   s.publicModel,
			Choices: choices,
		}
		if original.UsageMetadata != (usageMetadata{}) {
			chunk.Usage = &openai.Usage{
				PromptTokens:     s.promptTokens,
				CompletionTokens: s.completionTokens,
				TotalTokens:      s.totalTokens,
			}
		}
		encoded, err := json.Marshal(chunk)
		if err != nil {
			return nil, fmt.Errorf("encode OpenAI SSE data: %w", err)
		}
		output = append(output, sse.EncodeData(encoded)...)
	}
	return output, nil
}

func (s *Stream) allCandidatesFinished() bool {
	if len(s.seenCandidates) == 0 {
		return false
	}
	for index := range s.seenCandidates {
		if !s.finished[index] {
			return false
		}
	}
	return true
}
