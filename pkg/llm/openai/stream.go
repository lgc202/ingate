package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/sse"
)

// Stream 增量改写 OpenAI-compatible SSE 中的公开模型名
type Stream struct {
	decoder     sse.Decoder
	publicModel string
	seenChoices map[int]bool
	finished    map[int]bool
	failed      bool
	done        bool
}

// NewStream 创建 OpenAI-compatible SSE 转换器
func NewStream(publicModel string) (*Stream, error) {
	if strings.TrimSpace(publicModel) == "" {
		return nil, fmt.Errorf("%w: public model must not be empty", llm.ErrInvalidStream)
	}
	return &Stream{
		publicModel: publicModel,
		seenChoices: make(map[int]bool),
		finished:    make(map[int]bool),
	}, nil
}

// Push 转换一个任意大小和边界的上游网络分块
func (s *Stream) Push(chunk []byte) ([]byte, error) {
	events, err := s.decoder.Push(chunk)
	if err != nil {
		return nil, fmt.Errorf("%w: parse OpenAI SSE: %v", llm.ErrInvalidStream, err)
	}
	return s.transform(events)
}

// Finish 完成 SSE 转换；兼容未显式发送 [DONE] 但正常结束的 OpenAI-compatible 上游
func (s *Stream) Finish() ([]byte, error) {
	events, err := s.decoder.Finish()
	if err != nil {
		return nil, fmt.Errorf("%w: finish OpenAI SSE: %v", llm.ErrInvalidStream, err)
	}
	output, err := s.transform(events)
	if err != nil {
		return nil, err
	}
	if !s.done && !s.failed && !s.allChoicesFinished() {
		return nil, fmt.Errorf("%w: OpenAI stream ended before finish_reason or [DONE]", llm.ErrInvalidStream)
	}
	if !s.done {
		output = append(output, sse.EncodeData([]byte("[DONE]"))...)
		s.done = true
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
			return nil, fmt.Errorf("%w: received data after [DONE]", llm.ErrInvalidStream)
		}
		if bytes.Equal(data, []byte("[DONE]")) {
			output = append(output, sse.EncodeData([]byte("[DONE]"))...)
			s.done = true
			continue
		}
		if s.failed {
			return nil, fmt.Errorf("%w: received OpenAI data after an error event", llm.ErrInvalidStream)
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			return nil, fmt.Errorf("%w: decode OpenAI SSE data: %v", llm.ErrInvalidStream, err)
		}
		if _, isError := fields["error"]; isError {
			output = append(output, sse.EncodeData(TransformError(data, 502))...)
			s.failed = true
			continue
		}
		var marker struct {
			Choices []struct {
				Index        int             `json:"index"`
				FinishReason json.RawMessage `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(data, &marker); err != nil {
			return nil, fmt.Errorf("%w: decode OpenAI finish state: %v", llm.ErrInvalidStream, err)
		}
		for _, choice := range marker.Choices {
			s.seenChoices[choice.Index] = true
			if len(choice.FinishReason) > 0 && !bytes.Equal(bytes.TrimSpace(choice.FinishReason), []byte("null")) {
				s.finished[choice.Index] = true
			}
		}
		model, err := json.Marshal(s.publicModel)
		if err != nil {
			return nil, fmt.Errorf("encode public model: %w", err)
		}
		fields["model"] = model
		transformed, err := json.Marshal(fields)
		if err != nil {
			return nil, fmt.Errorf("encode OpenAI SSE data: %w", err)
		}
		output = append(output, sse.EncodeData(transformed)...)
	}
	return output, nil
}

func (s *Stream) allChoicesFinished() bool {
	if len(s.seenChoices) == 0 {
		return false
	}
	for index := range s.seenChoices {
		if !s.finished[index] {
			return false
		}
	}
	return true
}
