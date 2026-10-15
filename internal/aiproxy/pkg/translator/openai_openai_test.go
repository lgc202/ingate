package translator_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/endpointspec"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/translator"
)

func TestTranslateChatCompletionOpenAIToOpenAIPreservesFields(t *testing.T) {
	body := []byte(`{
		"model": "coding",
		"Model": "provider-specific",
		"messages": [{"role":"user","content":"<你好>","model":"nested"}],
		"stream": true,
		"stream_options": {"include_usage":false,"continuous_usage_stats":true},
		"extra": {"large":9007199254740993,"exponent":1e400,"decimal":1.2300,"empty":null}
	}`)
	request, err := endpointspec.ParseChatCompletions(body)
	if err != nil {
		t.Fatalf("ParseChatCompletions(provider extensions) error = %v", err)
	}
	want := decodeRequest(t, body)
	want["model"] = "code-v2"

	encoded, err := translator.TranslateChatCompletionOpenAIToOpenAI(body, "code-v2")
	if err != nil {
		t.Fatalf("TranslateChatCompletionOpenAIToOpenAI(code-v2) error = %v", err)
	}

	if diff := cmp.Diff(want, decodeRequest(t, encoded)); diff != "" {
		t.Errorf("TranslateChatCompletionOpenAIToOpenAI(code-v2) changed unrelated JSON values (-want +got):\n%s", diff)
	}
	if got := request.Model; got != "coding" {
		t.Errorf("ParseChatCompletions(original body).Model after translation = %q, want coding", got)
	}

	want["model"] = "other\"\\模型"

	other, err := translator.TranslateChatCompletionOpenAIToOpenAI(body, "other\"\\模型")
	if err != nil {
		t.Fatalf("TranslateChatCompletionOpenAIToOpenAI(escaped model) error = %v", err)
	}

	if diff := cmp.Diff(want, decodeRequest(t, other)); diff != "" {
		t.Errorf("TranslateChatCompletionOpenAIToOpenAI(escaped model) mismatch (-want +got):\n%s", diff)
	}
}

func TestTranslateChatCompletionOpenAIToOpenAIOwnsResult(t *testing.T) {
	body := []byte(" { \"model\" : \"coding\", \"messages\" : [] }\n")
	original := bytes.Clone(body)
	if _, err := endpointspec.ParseChatCompletions(body); err != nil {
		t.Fatalf("ParseChatCompletions(valid body) error = %v", err)
	}

	for _, override := range []string{"", "coding", "code-v2", "other"} {
		want := decodeRequest(t, original)
		if override != "" {
			want["model"] = override
		}

		encoded, err := translator.TranslateChatCompletionOpenAIToOpenAI(body, override)
		if err != nil {
			t.Fatalf("TranslateChatCompletionOpenAIToOpenAI(%q) error = %v", override, err)
		}

		if diff := cmp.Diff(want, decodeRequest(t, encoded)); diff != "" {
			t.Errorf("TranslateChatCompletionOpenAIToOpenAI(%q) mismatch (-want +got):\n%s", override, diff)
		}
		if (override == "" || override == "coding") && !bytes.Equal(encoded, original) {
			t.Errorf("TranslateChatCompletionOpenAIToOpenAI(%q) = %q, want original bytes %q", override, encoded, original)
		}
		if !bytes.Equal(body, original) {
			t.Errorf("TranslateChatCompletionOpenAIToOpenAI(%q) mutated source = %q, want %q", override, body, original)
		}

		for i := range encoded {
			encoded[i] = 'x'
		}

		if !bytes.Equal(body, original) {
			t.Errorf("TranslateChatCompletionOpenAIToOpenAI(%q) result aliases source = %q, want %q", override, body, original)
		}
	}

	want := decodeRequest(t, original)
	want["model"] = "code-v2"
	encoded, err := translator.TranslateChatCompletionOpenAIToOpenAI(body, "code-v2")
	if err != nil {
		t.Fatalf("TranslateChatCompletionOpenAIToOpenAI(code-v2) error = %v", err)
	}

	for i := range body {
		body[i] = 'x'
	}

	if diff := cmp.Diff(want, decodeRequest(t, encoded)); diff != "" {
		t.Errorf("TranslateChatCompletionOpenAIToOpenAI() retained mutable source (-want +got):\n%s", diff)
	}
}

func TestTranslateChatCompletionOpenAIToOpenAIRejectsInvalidOverride(t *testing.T) {
	body := []byte(`{"model":"coding"}`)
	_, err := endpointspec.ParseChatCompletions(body)
	if err != nil {
		t.Fatalf("ParseChatCompletions(valid body) error = %v", err)
	}

	encoded, err := translator.TranslateChatCompletionOpenAIToOpenAI(body, "\xff")

	if err == nil || encoded != nil {
		t.Errorf("TranslateChatCompletionOpenAIToOpenAI(invalid UTF-8) = (%q, %v), want nil body and error", encoded, err)
	}
}

func decodeRequest(t *testing.T, body []byte) map[string]any {
	t.Helper()
	if !json.Valid(body) {
		t.Fatalf("TranslateChatCompletionOpenAIToOpenAI() body = %q, want a single valid JSON document", body)
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var fields map[string]any
	if err := decoder.Decode(&fields); err != nil {
		t.Fatalf("Decode(request body) error = %v", err)
	}

	return fields
}
