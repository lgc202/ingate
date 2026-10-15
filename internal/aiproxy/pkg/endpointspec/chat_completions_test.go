package endpointspec_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/endpointspec"
)

func TestParseChatCompletions(t *testing.T) {
	for _, tt := range []struct {
		name  string
		body  string
		model string
	}{
		{name: "model", body: `{"model":"coding","messages":[]}`, model: "coding"},
		{name: "escaped model", body: `{"model":"cod\u0069ng"}`, model: "coding"},
		{name: "exact whitespace", body: `{"model":" Coding "}`, model: " Coding "},
		{name: "escaped key", body: `{"\u006dodel":"coding"}`, model: "coding"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request, err := endpointspec.ParseChatCompletions([]byte(tt.body))
			if err != nil {
				t.Fatalf("ParseChatCompletions(%q) error = %v", tt.body, err)
			}

			if got := request.Model; got != tt.model {
				t.Errorf("ParseChatCompletions(%q).Model = %q, want %q", tt.body, got, tt.model)
			}
		})
	}
}

func TestParseChatCompletionsRejectsInvalidBody(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "empty"},
		{name: "invalid JSON", body: `{"model":"coding"`},
		{name: "null", body: `null`},
		{name: "array", body: `[{"model":"coding"}]`},
		{name: "string", body: `"coding"`},
		{name: "missing model", body: `{"messages":[]}`},
		{name: "wrong case", body: `{"Model":"coding"}`},
		{name: "null model", body: `{"model":null}`},
		{name: "empty model", body: `{"model":""}`},
		{name: "number model", body: `{"model":42}`},
		{name: "boolean model", body: `{"model":true}`},
		{name: "object model", body: `{"model":{}}`},
		{name: "multiple documents", body: `{"model":"coding"} {}`},
		{name: "duplicate model", body: `{"model":"coding","model":"other"}`},
		{name: "duplicate escaped model", body: `{"model":"coding","\u006dodel":"other"}`},
		{name: "duplicate extension", body: `{"model":"coding","extra":1,"extra":2}`},
		{name: "invalid UTF-8", body: "{\"model\":\"\xff\"}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request, err := endpointspec.ParseChatCompletions([]byte(tt.body))

			if err == nil || request != nil {
				t.Errorf("ParseChatCompletions(%q) = (%+v, %v), want nil request and error", tt.body, request, err)
			}
		})
	}
}

func TestParseChatCompletionsPreservesErrorCause(t *testing.T) {
	_, err := endpointspec.ParseChatCompletions([]byte(`{"model":42}`))

	var typeError *json.UnmarshalTypeError
	if !errors.As(err, &typeError) {
		t.Errorf("ParseChatCompletions(number model) error = %v, want wrapped *json.UnmarshalTypeError", err)
	}
}

func TestParseChatCompletionsOwnsFields(t *testing.T) {
	body := []byte(`{"model":"cod\u0069ng","messages":[]}`)
	request, err := endpointspec.ParseChatCompletions(body)
	if err != nil {
		t.Fatalf("ParseChatCompletions(valid body) error = %v", err)
	}

	for i := range body {
		body[i] = 'x'
	}

	if request.Model != "coding" {
		t.Errorf("ParseChatCompletions() model after source mutation = %q, want coding", request.Model)
	}
}
