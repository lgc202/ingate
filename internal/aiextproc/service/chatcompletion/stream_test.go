package chatcompletion

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAnthropicStreamConvertMetadataChanges 验证正文转换与元数据更新使用独立的返回约定。
func TestAnthropicStreamConvertMetadataChanges(t *testing.T) {
	stream := NewAnthropicStream("client-model")
	start := "event: message_start\ndata: " +
		`{"type":"message_start","message":{"id":"message-1","model":"provider-model","usage":{"input_tokens":2,"output_tokens":0}}}` + "\n\n"
	output, metadata, metadataChanged, err := stream.Convert([]byte(start), false)
	if err != nil {
		t.Fatalf("Convert(message_start) error = %v, want nil", err)
	}
	if len(output) != 0 || !metadataChanged || metadata.ResponseModel != "provider-model" || !metadata.Usage.Found {
		t.Fatalf(
			"Convert(message_start) = (%q, %+v, %t), want no output and updated model usage",
			output,
			metadata,
			metadataChanged,
		)
	}
	initialMetadata := metadata

	fragment := "event: content_block_delta\ndata: " +
		`{"type":"content_block_delta","delta":{"type":"text_delta","text":"hel`
	output, metadata, metadataChanged, err = stream.Convert([]byte(fragment), false)
	if err != nil {
		t.Fatalf("Convert(partial text event) error = %v, want nil", err)
	}
	if len(output) != 0 || metadataChanged || metadata != initialMetadata {
		t.Errorf(
			"Convert(partial text event) = (%q, %+v, %t), want no output and metadata %+v unchanged",
			output,
			metadata,
			metadataChanged,
			initialMetadata,
		)
	}

	output, metadata, metadataChanged, err = stream.Convert([]byte("lo\"}}\n\n"), false)
	if err != nil {
		t.Fatalf("Convert(remaining text event) error = %v, want nil", err)
	}
	if metadataChanged || metadata != initialMetadata {
		t.Errorf(
			"Convert(text event) metadata = (%+v, %t), want (%+v, false)",
			metadata,
			metadataChanged,
			initialMetadata,
		)
	}
	data := strings.TrimPrefix(string(output), "data: ")
	data = strings.TrimSpace(data)
	var chunk openAIStreamResponse
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		t.Fatalf("Convert(text event) output = %q: decode error = %v, want OpenAI SSE data", output, err)
	}
	if chunk.Model != "client-model" || len(chunk.Choices) != 1 ||
		chunk.Choices[0].Delta.Content == nil || *chunk.Choices[0].Delta.Content != "hello" {
		t.Errorf("Convert(text event) chunk = %+v, want client-model with hello text", chunk)
	}

	output, metadata, metadataChanged, err = stream.Convert([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"), false)
	if err != nil {
		t.Fatalf("Convert(message_stop) error = %v, want nil", err)
	}
	if len(output) == 0 || !metadataChanged || !metadata.Usage.Final {
		t.Errorf(
			"Convert(message_stop) = (%q, %+v, %t), want output and final usage update",
			output,
			metadata,
			metadataChanged,
		)
	}
}

// TestOpenAIStreamConvertMetadataChanges 验证正文分片保留元数据，并在流结束时更新最终用量。
func TestOpenAIStreamConvertMetadataChanges(t *testing.T) {
	stream := NewOpenAIStream("client-model")
	usage := "data: " + `{"model":"provider-model","usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}` + "\n\n"
	output, metadata, metadataChanged, err := stream.Convert([]byte(usage), false)
	if err != nil {
		t.Fatalf("Convert(usage event) error = %v, want nil", err)
	}
	if len(output) == 0 || !metadataChanged || metadata.Usage.TotalTokens != 5 || metadata.Usage.Final {
		t.Fatalf(
			"Convert(usage event) = (%q, %+v, %t), want output and non-final usage update",
			output,
			metadata,
			metadataChanged,
		)
	}
	initialMetadata := metadata

	fragment := `data: {"choices":[{"delta":{"content":"hel`
	output, metadata, metadataChanged, err = stream.Convert([]byte(fragment), false)
	if err != nil {
		t.Fatalf("Convert(partial text event) error = %v, want nil", err)
	}
	if len(output) != 0 || metadataChanged || metadata != initialMetadata {
		t.Errorf(
			"Convert(partial text event) = (%q, %+v, %t), want no output and metadata %+v unchanged",
			output,
			metadata,
			metadataChanged,
			initialMetadata,
		)
	}

	output, metadata, metadataChanged, err = stream.Convert([]byte("lo\"}}]}\n\n"), false)
	if err != nil {
		t.Fatalf("Convert(remaining text event) error = %v, want nil", err)
	}
	if len(output) == 0 || metadataChanged || metadata != initialMetadata {
		t.Errorf(
			"Convert(text event) = (%q, %+v, %t), want output and metadata %+v unchanged",
			output,
			metadata,
			metadataChanged,
			initialMetadata,
		)
	}

	output, metadata, metadataChanged, err = stream.Convert([]byte("data: [DONE]\n\n"), false)
	if err != nil {
		t.Fatalf("Convert(completion event) error = %v, want nil", err)
	}
	if len(output) == 0 || !metadataChanged || !metadata.Usage.Final || metadata.Usage.TotalTokens != 5 {
		t.Errorf(
			"Convert(completion event) = (%q, %+v, %t), want output and final usage of 5 tokens",
			output,
			metadata,
			metadataChanged,
		)
	}
}
