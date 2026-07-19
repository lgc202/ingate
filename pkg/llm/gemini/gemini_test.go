package gemini

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/openai"
	"github.com/lgc202/ingate/pkg/llm/sse"
)

func TestEndpointPath(t *testing.T) {
	got, err := EndpointPath("publishers/google/models/gemini-pro", true)
	if err != nil {
		t.Fatalf("EndpointPath(..., true) returned error: %v", err)
	}
	want := "/models/publishers%2Fgoogle%2Fmodels%2Fgemini-pro:streamGenerateContent?alt=sse"
	if got != want {
		t.Errorf("EndpointPath(..., true) = %q, want %q", got, want)
	}
}

func TestStream_RejectsUnfinishedCandidate(t *testing.T) {
	stream, err := NewStream("gemini-public")
	if err != nil {
		t.Fatalf("NewStream(gemini-public) returned error: %v", err)
	}
	_, err = stream.Push([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"partial\"}]},\"index\":0}]}\n\n"))
	if err != nil {
		t.Fatalf("Stream.Push(partial candidate) returned error: %v", err)
	}
	if _, err := stream.Finish(); !errors.Is(err, llm.ErrInvalidStream) {
		t.Errorf("Stream.Finish() error = %v, want errors.Is(_, ErrInvalidStream)", err)
	}
}

func TestStream_WrapsSSEBufferLimit(t *testing.T) {
	stream, err := NewStream("gemini-public")
	if err != nil {
		t.Fatalf("NewStream(gemini-public) returned error: %v", err)
	}
	input := []byte("data: " + strings.Repeat("x", sse.MaxBufferedBytes))
	if _, err := stream.Push(input); !errors.Is(err, llm.ErrInvalidStream) {
		t.Errorf("Stream.Push(%d-byte unfinished event) error = %v, want errors.Is(_, ErrInvalidStream)", len(input), err)
	}
}

func TestTransformRequest(t *testing.T) {
	body := []byte(`{
		"model":"gemini-public",
		"messages":[
			{"role":"system","content":"be concise"},
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"}
		],
		"stream":true,
		"temperature":0,
		"top_p":0.9,
		"max_tokens":256,
		"stop":"END"
	}`)
	original, err := openai.DecodeRequest(body)
	if err != nil {
		t.Fatalf("openai.DecodeRequest(%s) returned error: %v", body, err)
	}
	got, err := TransformRequest(original)
	if err != nil {
		t.Fatalf("TransformRequest(%s) returned error: %v", body, err)
	}
	var transformed requestBody
	if err := json.Unmarshal(got, &transformed); err != nil {
		t.Fatalf("json.Unmarshal(TransformRequest result) returned error: %v", err)
	}
	if transformed.SystemInstruction == nil || len(transformed.SystemInstruction.Parts) != 1 || *transformed.SystemInstruction.Parts[0].Text != "be concise" {
		t.Errorf("TransformRequest systemInstruction = %#v, want be concise", transformed.SystemInstruction)
	}
	if len(transformed.Contents) != 2 || transformed.Contents[0].Role != "user" || transformed.Contents[1].Role != "model" {
		t.Errorf("TransformRequest contents = %#v, want user/model roles", transformed.Contents)
	}
	if transformed.GenerationConfig == nil || transformed.GenerationConfig.MaxOutputTokens == nil || *transformed.GenerationConfig.MaxOutputTokens != 256 {
		t.Errorf("TransformRequest generationConfig = %#v, want maxOutputTokens 256", transformed.GenerationConfig)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(got, &fields); err != nil {
		t.Fatalf("json.Unmarshal(TransformRequest result fields) returned error: %v", err)
	}
	if _, ok := fields["model"]; ok {
		t.Error("TransformRequest unexpectedly included model in Gemini request body")
	}
	if _, ok := fields["stream"]; ok {
		t.Error("TransformRequest unexpectedly included stream in Gemini request body")
	}
}

func TestTransformRequest_RejectsLateSystemMessage(t *testing.T) {
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"x"},{"role":"system","content":"late"}]}`)
	_, err := openai.DecodeRequest(body)
	if !errors.Is(err, llm.ErrUnsupportedFeature) {
		t.Errorf("openai.DecodeRequest(%s) error = %v, want errors.Is(_, ErrUnsupportedFeature)", body, err)
	}
}

func TestTransformResponse(t *testing.T) {
	body := []byte(`{
		"candidates":[{
			"content":{"parts":[{"text":"hello "},{"text":"world"}],"role":"model"},
			"finishReason":"MAX_TOKENS","index":0
		}],
		"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":2,"totalTokenCount":10},
		"modelVersion":"gemini-2.5-flash","responseId":"resp_1"
	}`)
	got, err := TransformResponse(body, "gemini-public")
	if err != nil {
		t.Fatalf("TransformResponse(%s, gemini-public) returned error: %v", body, err)
	}
	var completion openai.ChatCompletion
	if err := json.Unmarshal(got, &completion); err != nil {
		t.Fatalf("json.Unmarshal(TransformResponse result) returned error: %v", err)
	}
	if completion.ID != "resp_1" || completion.Model != "gemini-public" {
		t.Errorf("TransformResponse metadata = %#v, want resp_1/gemini-public", completion)
	}
	if len(completion.Choices) != 1 || completion.Choices[0].Message.Content != "hello world" {
		t.Errorf("TransformResponse choices = %#v, want concatenated text", completion.Choices)
	}
	if completion.Choices[0].FinishReason == nil || *completion.Choices[0].FinishReason != openai.FinishReasonLength {
		t.Errorf("TransformResponse finish_reason = %v, want length", completion.Choices[0].FinishReason)
	}
	if completion.Usage == nil || completion.Usage.TotalTokens != 10 {
		t.Errorf("TransformResponse usage = %#v, want total_tokens 10", completion.Usage)
	}
}

func TestTransformResponse_MapsSafetyFinishReason(t *testing.T) {
	body := []byte(`{"candidates":[{"content":{"parts":[],"role":"model"},"finishReason":"SAFETY","index":0}]}`)
	got, err := TransformResponse(body, "gemini-public")
	if err != nil {
		t.Fatalf("TransformResponse(%s, gemini-public) returned error: %v", body, err)
	}
	var completion openai.ChatCompletion
	if err := json.Unmarshal(got, &completion); err != nil {
		t.Fatalf("json.Unmarshal(TransformResponse result) returned error: %v", err)
	}
	if completion.Choices[0].FinishReason == nil || *completion.Choices[0].FinishReason != openai.FinishReasonContentFilter {
		t.Errorf("TransformResponse finish_reason = %v, want content_filter", completion.Choices[0].FinishReason)
	}
}

func TestTransformError(t *testing.T) {
	body := TransformError([]byte(`{"error":{"code":400,"message":"bad request","status":"INVALID_ARGUMENT"}}`), 400)
	var envelope openai.ErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(TransformError result) returned error: %v", err)
	}
	want := openai.ErrorDetail{Message: "bad request", Type: "invalid_request_error", Code: "INVALID_ARGUMENT"}
	if envelope.Error != want {
		t.Errorf("TransformError(...) = %#v, want %#v", envelope.Error, want)
	}
}

func TestStream_ConvertsChunksAndUsage(t *testing.T) {
	stream, err := NewStream("gemini-public")
	if err != nil {
		t.Fatalf("NewStream(gemini-public) returned error: %v", err)
	}
	input := []byte(
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hel\"}],\"role\":\"model\"},\"index\":0}],\"responseId\":\"resp_1\"}\n\n" +
			"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"lo\"}],\"role\":\"model\"},\"finishReason\":\"STOP\",\"index\":0}],\"usageMetadata\":{\"promptTokenCount\":5,\"candidatesTokenCount\":2,\"totalTokenCount\":7},\"responseId\":\"resp_1\"}\n\n")

	var output []byte
	for _, value := range input {
		chunk, err := stream.Push([]byte{value})
		if err != nil {
			t.Fatalf("Stream.Push(%q) returned error: %v", value, err)
		}
		output = append(output, chunk...)
	}
	last, err := stream.Finish()
	if err != nil {
		t.Fatalf("Stream.Finish() returned error: %v", err)
	}
	output = append(output, last...)

	events := decodeEvents(t, output)
	if len(events) != 3 {
		t.Fatalf("converted SSE event count = %d, want 3; output=%s", len(events), output)
	}
	var first openai.ChatCompletionChunk
	if err := json.Unmarshal(events[0].Data, &first); err != nil {
		t.Fatalf("json.Unmarshal(first chunk) returned error: %v", err)
	}
	if first.ID != "resp_1" || first.Model != "gemini-public" || first.Choices[0].Delta.Role != openai.RoleAssistant || *first.Choices[0].Delta.Content != "hel" {
		t.Errorf("first chunk = %#v, want role assistant and text hel", first)
	}
	var second openai.ChatCompletionChunk
	if err := json.Unmarshal(events[1].Data, &second); err != nil {
		t.Fatalf("json.Unmarshal(second chunk) returned error: %v", err)
	}
	if second.Choices[0].Delta.Role != "" || second.Choices[0].FinishReason == nil || *second.Choices[0].FinishReason != openai.FinishReasonStop {
		t.Errorf("second chunk choice = %#v, want no role and finish stop", second.Choices[0])
	}
	if second.Usage == nil || second.Usage.TotalTokens != 7 {
		t.Errorf("second chunk usage = %#v, want total_tokens 7", second.Usage)
	}
	if string(events[2].Data) != "[DONE]" {
		t.Errorf("final SSE data = %q, want [DONE]", events[2].Data)
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
