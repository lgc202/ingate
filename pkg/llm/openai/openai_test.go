package openai

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/sse"
)

func TestTransformRequest_RejectsUnknownFields(t *testing.T) {
	body := []byte(`{
		"model":"public-model",
		"messages":[{"role":"user","content":"hello","name":"caller"}],
		"frequency_penalty":0.4,
		"vendor_option":{"enabled":true}
	}`)
	if _, err := DecodeRequest(body); !errors.Is(err, llm.ErrUnsupportedFeature) {
		t.Errorf("DecodeRequest(%s) error = %v, want errors.Is(_, ErrUnsupportedFeature)", body, err)
	}
}

func TestTransformRequest_ReusesDecodedRequestAndPreservesFields(t *testing.T) {
	body := []byte(`{
		"model":"public-model",
		"messages":[{"role":"user","content":"hello"}],
		"temperature":0,
		"stop":"END"
	}`)
	request, err := DecodeRequest(body)
	if err != nil {
		t.Fatalf("DecodeRequest(%s) returned error: %v", body, err)
	}
	got, err := TransformRequest(request, "upstream-model")
	if err != nil {
		t.Fatalf("TransformRequest(request, upstream-model) returned error: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(got, &fields); err != nil {
		t.Fatalf("json.Unmarshal(TransformRequest result) returned error: %v", err)
	}
	if string(fields["model"]) != `"upstream-model"` {
		t.Errorf("TransformRequest model = %s, want upstream-model", fields["model"])
	}
	if string(fields["temperature"]) != "0" {
		t.Errorf("TransformRequest temperature = %s, want 0", fields["temperature"])
	}
	if string(fields["stop"]) != `"END"` {
		t.Errorf("TransformRequest stop = %s, want original string form", fields["stop"])
	}
}

func TestStream_RejectsTruncatedSequence(t *testing.T) {
	stream, err := NewStream("public-model")
	if err != nil {
		t.Fatalf("NewStream(public-model) returned error: %v", err)
	}
	_, err = stream.Push([]byte("data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))
	if err != nil {
		t.Fatalf("Stream.Push(partial chunk) returned error: %v", err)
	}
	if _, err := stream.Finish(); !errors.Is(err, llm.ErrInvalidStream) {
		t.Errorf("Stream.Finish() error = %v, want errors.Is(_, ErrInvalidStream)", err)
	}
}

func TestStream_WrapsSSEBufferLimit(t *testing.T) {
	stream, err := NewStream("public-model")
	if err != nil {
		t.Fatalf("NewStream(public-model) returned error: %v", err)
	}
	input := []byte("data: " + strings.Repeat("x", sse.MaxBufferedBytes))
	if _, err := stream.Push(input); !errors.Is(err, llm.ErrInvalidStream) {
		t.Errorf("Stream.Push(%d-byte unfinished event) error = %v, want errors.Is(_, ErrInvalidStream)", len(input), err)
	}
}

func TestStream_NormalizesErrorAndCompletes(t *testing.T) {
	stream, err := NewStream("public-model")
	if err != nil {
		t.Fatalf("NewStream(public-model) returned error: %v", err)
	}
	output, err := stream.Push([]byte("data: {\"error\":{\"message\":\"upstream failed\",\"type\":\"server_error\"}}\n\n"))
	if err != nil {
		t.Fatalf("Stream.Push(error event) returned error: %v", err)
	}
	last, err := stream.Finish()
	if err != nil {
		t.Fatalf("Stream.Finish() returned error: %v", err)
	}
	output = append(output, last...)
	events := decodeEvents(t, output)
	if len(events) != 2 || string(events[1].Data) != "[DONE]" {
		t.Fatalf("converted error SSE = %s, want error event followed by [DONE]", output)
	}
	var envelope ErrorEnvelope
	if err := json.Unmarshal(events[0].Data, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(error event) returned error: %v", err)
	}
	if envelope.Error.Message != "upstream failed" || envelope.Error.Type != "server_error" {
		t.Errorf("converted error = %#v, want upstream failed/server_error", envelope.Error)
	}
}

func TestTransformResponse_ChangesPublicModelAndPreservesFields(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-1","model":"vendor-model","choices":[],"system_fingerprint":"fp-1"}`)
	got, err := TransformResponse(body, "public-model")
	if err != nil {
		t.Fatalf("TransformResponse(%s, public-model) returned error: %v", body, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(got, &fields); err != nil {
		t.Fatalf("json.Unmarshal(TransformResponse result) returned error: %v", err)
	}
	if string(fields["model"]) != `"public-model"` {
		t.Errorf("TransformResponse model = %s, want public-model", fields["model"])
	}
	if string(fields["system_fingerprint"]) != `"fp-1"` {
		t.Errorf("TransformResponse system_fingerprint = %s, want fp-1", fields["system_fingerprint"])
	}
}

func TestTransformError(t *testing.T) {
	body := TransformError([]byte(`{"error":{"message":"bad model","type":"invalid_request_error","param":"model","code":400}}`), 400)
	var envelope ErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(TransformError result) returned error: %v", err)
	}
	param := "model"
	want := ErrorDetail{Message: "bad model", Type: "invalid_request_error", Param: &param, Code: "400"}
	if !reflect.DeepEqual(envelope.Error, want) {
		t.Errorf("TransformError(...) = %#v, want %#v", envelope.Error, want)
	}
}

func TestStream_ArbitraryChunksAndImplicitDone(t *testing.T) {
	stream, err := NewStream("public-model")
	if err != nil {
		t.Fatalf("NewStream(public-model) returned error: %v", err)
	}
	input := []byte("data: {\"id\":\"1\",\"model\":\"vendor\",\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}]}\n\n")
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
	if len(events) != 2 {
		t.Fatalf("converted SSE event count = %d, want 2; output=%s", len(events), output)
	}
	var chunk map[string]json.RawMessage
	if err := json.Unmarshal(events[0].Data, &chunk); err != nil {
		t.Fatalf("json.Unmarshal(first SSE data) returned error: %v", err)
	}
	if string(chunk["model"]) != `"public-model"` {
		t.Errorf("converted SSE model = %s, want public-model", chunk["model"])
	}
	if string(events[1].Data) != "[DONE]" {
		t.Errorf("final SSE data = %q, want [DONE]", events[1].Data)
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
