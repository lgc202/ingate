package anthropic

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/sse"
)

func TestTransformRequest(t *testing.T) {
	body := []byte(`{
		"model":"claude-public",
		"messages":[
			{"role":"system","content":"be concise"},
			{"role":"system","content":"answer in Chinese"},
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"}
		],
		"stream":true,
		"temperature":0,
		"top_p":0.9,
		"stop":["END"]
	}`)
	got, err := TransformRequest(body, "claude-sonnet-4")
	if err != nil {
		t.Fatalf("TransformRequest(%s, claude-sonnet-4) returned error: %v", body, err)
	}

	var request map[string]json.RawMessage
	if err := json.Unmarshal(got, &request); err != nil {
		t.Fatalf("json.Unmarshal(TransformRequest result) returned error: %v", err)
	}
	checks := map[string]string{
		"model":       `"claude-sonnet-4"`,
		"system":      `"be concise\n\nanswer in Chinese"`,
		"max_tokens":  `4096`,
		"stream":      `true`,
		"temperature": `0`,
		"top_p":       `0.9`,
	}
	for field, want := range checks {
		if string(request[field]) != want {
			t.Errorf("TransformRequest field %s = %s, want %s", field, request[field], want)
		}
	}
	var messages []message
	if err := json.Unmarshal(request["messages"], &messages); err != nil {
		t.Fatalf("json.Unmarshal(messages) returned error: %v", err)
	}
	if len(messages) != 2 || messages[0].Role != llm.RoleUser || messages[1].Role != llm.RoleAssistant {
		t.Errorf("TransformRequest messages = %#v, want user and assistant messages", messages)
	}
}

func TestTransformRequest_RejectsUnknownPortableField(t *testing.T) {
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"x"}],"frequency_penalty":1}`)
	_, err := TransformRequest(body, "claude")
	if !errors.Is(err, llm.ErrUnsupportedFeature) {
		t.Errorf("TransformRequest(%s, claude) error = %v, want errors.Is(_, ErrUnsupportedFeature)", body, err)
	}
}

func TestTransformRequest_RejectsLateSystemMessage(t *testing.T) {
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"x"},{"role":"system","content":"late"}]}`)
	_, err := TransformRequest(body, "claude")
	if !errors.Is(err, llm.ErrUnsupportedFeature) {
		t.Errorf("TransformRequest(%s, claude) error = %v, want errors.Is(_, ErrUnsupportedFeature)", body, err)
	}
}

func TestTransformResponse(t *testing.T) {
	body := []byte(`{
		"id":"msg_123","type":"message","role":"assistant","model":"claude-sonnet-4",
		"content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}],
		"stop_reason":"max_tokens","usage":{"input_tokens":12,"output_tokens":3}
	}`)
	got, err := TransformResponse(body, "claude-public")
	if err != nil {
		t.Fatalf("TransformResponse(%s, claude-public) returned error: %v", body, err)
	}
	var completion llm.ChatCompletion
	if err := json.Unmarshal(got, &completion); err != nil {
		t.Fatalf("json.Unmarshal(TransformResponse result) returned error: %v", err)
	}
	if completion.ID != "msg_123" || completion.Model != "claude-public" || completion.Object != llm.ObjectChatCompletion {
		t.Errorf("TransformResponse metadata = %#v, want msg_123/claude-public/chat.completion", completion)
	}
	if len(completion.Choices) != 1 || completion.Choices[0].Message.Content != "hello world" {
		t.Errorf("TransformResponse choices = %#v, want concatenated text", completion.Choices)
	}
	if completion.Choices[0].FinishReason == nil || *completion.Choices[0].FinishReason != llm.FinishReasonLength {
		t.Errorf("TransformResponse finish_reason = %v, want length", completion.Choices[0].FinishReason)
	}
	if completion.Usage == nil || completion.Usage.TotalTokens != 15 {
		t.Errorf("TransformResponse usage = %#v, want total_tokens 15", completion.Usage)
	}
}

func TestTransformError(t *testing.T) {
	body := TransformError([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"bad request"}}`), 400)
	var envelope llm.ErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(TransformError result) returned error: %v", err)
	}
	want := llm.APIError{Message: "bad request", Type: "invalid_request_error"}
	if envelope.Error != want {
		t.Errorf("TransformError(...) = %#v, want %#v", envelope.Error, want)
	}
}

func TestStream_ConvertsStateAndUsageAcrossChunks(t *testing.T) {
	stream, err := NewStream("claude-public")
	if err != nil {
		t.Fatalf("NewStream(claude-public) returned error: %v", err)
	}
	input := []byte(
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n" +
			"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
			"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":3}}\n\n" +
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")

	var output []byte
	for start := 0; start < len(input); start += 7 {
		end := min(start+7, len(input))
		chunk, err := stream.Push(input[start:end])
		if err != nil {
			t.Fatalf("Stream.Push(input[%d:%d]) returned error: %v", start, end, err)
		}
		output = append(output, chunk...)
	}
	last, err := stream.Finish()
	if err != nil {
		t.Fatalf("Stream.Finish() returned error: %v", err)
	}
	output = append(output, last...)

	events := decodeEvents(t, output)
	if len(events) != 5 {
		t.Fatalf("converted SSE event count = %d, want 5; output=%s", len(events), output)
	}
	var startChunk llm.ChatCompletionChunk
	if err := json.Unmarshal(events[0].Data, &startChunk); err != nil {
		t.Fatalf("json.Unmarshal(message_start chunk) returned error: %v", err)
	}
	if startChunk.ID != "msg_1" || startChunk.Model != "claude-public" || startChunk.Choices[0].Delta.Role != llm.RoleAssistant {
		t.Errorf("message_start chunk = %#v, want assistant role and public metadata", startChunk)
	}
	var textChunk llm.ChatCompletionChunk
	if err := json.Unmarshal(events[1].Data, &textChunk); err != nil {
		t.Fatalf("json.Unmarshal(text chunk) returned error: %v", err)
	}
	if textChunk.Choices[0].Delta.Content == nil || *textChunk.Choices[0].Delta.Content != "hello" {
		t.Errorf("text chunk delta = %#v, want hello", textChunk.Choices[0].Delta)
	}
	var finishChunk llm.ChatCompletionChunk
	if err := json.Unmarshal(events[2].Data, &finishChunk); err != nil {
		t.Fatalf("json.Unmarshal(finish chunk) returned error: %v", err)
	}
	if finishChunk.Choices[0].FinishReason == nil || *finishChunk.Choices[0].FinishReason != llm.FinishReasonStop {
		t.Errorf("finish chunk reason = %v, want stop", finishChunk.Choices[0].FinishReason)
	}
	var usageChunk llm.ChatCompletionChunk
	if err := json.Unmarshal(events[3].Data, &usageChunk); err != nil {
		t.Fatalf("json.Unmarshal(usage chunk) returned error: %v", err)
	}
	if usageChunk.Usage == nil || usageChunk.Usage.PromptTokens != 10 || usageChunk.Usage.CompletionTokens != 3 || usageChunk.Usage.TotalTokens != 13 {
		t.Errorf("usage chunk = %#v, want 10/3/13", usageChunk.Usage)
	}
	if string(events[4].Data) != "[DONE]" {
		t.Errorf("final SSE data = %q, want [DONE]", events[4].Data)
	}
}

func TestStream_RejectsTruncatedSequence(t *testing.T) {
	stream, err := NewStream("claude-public")
	if err != nil {
		t.Fatalf("NewStream(claude-public) returned error: %v", err)
	}
	_, err = stream.Push([]byte("data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"usage\":{}}}\n\n"))
	if err != nil {
		t.Fatalf("Stream.Push(message_start) returned error: %v", err)
	}
	if _, err := stream.Finish(); !errors.Is(err, llm.ErrInvalidStream) {
		t.Errorf("Stream.Finish() error = %v, want errors.Is(_, ErrInvalidStream)", err)
	}
}

func TestStream_WrapsSSEBufferLimit(t *testing.T) {
	stream, err := NewStream("claude-public")
	if err != nil {
		t.Fatalf("NewStream(claude-public) returned error: %v", err)
	}
	input := []byte("data: " + strings.Repeat("x", sse.MaxBufferedBytes))
	if _, err := stream.Push(input); !errors.Is(err, llm.ErrInvalidStream) {
		t.Errorf("Stream.Push(%d-byte unfinished event) error = %v, want errors.Is(_, ErrInvalidStream)", len(input), err)
	}
}

func decodeEvents(t *testing.T, data []byte) []sse.Event {
	t.Helper()
	var decoder sse.Decoder
	events, err := decoder.Push(data)
	if err != nil {
		t.Fatalf("sse.Decoder.Push(%q) returned error: %v", data, err)
	}
	last, err := decoder.Finish()
	if err != nil {
		t.Fatalf("sse.Decoder.Finish() returned error: %v", err)
	}
	return append(events, last...)
}
