package openai

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lgc202/ingate/pkg/llm"
)

func TestDecodeRequest(t *testing.T) {
	body := []byte(`{
		"model":"public-model",
		"messages":[
			{"role":"system","content":"be concise"},
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"}
		],
		"stream":true,
		"temperature":0,
		"top_p":0.8,
		"max_tokens":128,
		"stop":"END"
	}`)

	got, err := DecodeRequest(body)
	if err != nil {
		t.Fatalf("DecodeRequest(%s) returned error: %v", body, err)
	}
	got.fields = nil
	want := Request{
		Model: "public-model",
		Messages: []Message{
			{Role: RoleSystem, Content: "be concise"},
			{Role: RoleUser, Content: "hello"},
			{Role: RoleAssistant, Content: "hi"},
		},
		Stream:      boolPointer(true),
		Temperature: floatPointer(0),
		TopP:        floatPointer(0.8),
		MaxTokens:   intPointer(128),
		Stop:        []string{"END"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DecodeRequest(%s) = %#v, want %#v", body, got, want)
	}
	if !got.Streaming() {
		t.Errorf("Request.Streaming() = false, want true")
	}
}

func TestDecodeRequest_RejectsUnknownFields(t *testing.T) {
	body := []byte(`{
		"model":"m",
		"messages":[{"role":"user","content":"hello","name":"client"}],
		"frequency_penalty":0.2
	}`)
	if _, err := DecodeRequest(body); !errors.Is(err, llm.ErrUnsupportedFeature) {
		t.Errorf("DecodeRequest(%s) error = %v, want errors.Is(_, ErrUnsupportedFeature)", body, err)
	}
}

func TestDecodeRequest_RejectsLateSystemMessage(t *testing.T) {
	body := []byte(`{
		"model":"m",
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"system","content":"answer briefly"}
		]
	}`)
	if _, err := DecodeRequest(body); !errors.Is(err, llm.ErrUnsupportedFeature) {
		t.Errorf("DecodeRequest(%s) error = %v, want errors.Is(_, ErrUnsupportedFeature)", body, err)
	}
}

func TestDecodeRequest_RejectsInvalidOrUnsupportedInput(t *testing.T) {
	tests := []struct {
		name string
		body string
		want error
	}{
		{name: "array body", body: `[]`, want: llm.ErrInvalidRequest},
		{name: "missing model", body: `{"messages":[{"role":"user","content":"x"}]}`, want: llm.ErrInvalidRequest},
		{name: "null content", body: `{"model":"m","messages":[{"role":"user","content":null}]}`, want: llm.ErrInvalidRequest},
		{name: "multimodal content", body: `{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"x"}]}]}`, want: llm.ErrUnsupportedFeature},
		{name: "tool role", body: `{"model":"m","messages":[{"role":"tool","content":"x"}]}`, want: llm.ErrUnsupportedFeature},
		{name: "tools", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"tools":[]}`, want: llm.ErrUnsupportedFeature},
		{name: "function call", body: `{"model":"m","messages":[{"role":"assistant","content":"","function_call":{"name":"x"}}]}`, want: llm.ErrUnsupportedFeature},
		{name: "null stream", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"stream":null}`, want: llm.ErrInvalidRequest},
		{name: "temperature range", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"temperature":2.1}`, want: llm.ErrInvalidRequest},
		{name: "top p type", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"top_p":"1"}`, want: llm.ErrInvalidRequest},
		{name: "fractional max tokens", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"max_tokens":1.5}`, want: llm.ErrInvalidRequest},
		{name: "zero max tokens", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"max_tokens":0}`, want: llm.ErrInvalidRequest},
		{name: "invalid stop", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"stop":{"value":"x"}}`, want: llm.ErrInvalidRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeRequest([]byte(test.body))
			if !errors.Is(err, test.want) {
				t.Errorf("DecodeRequest(%s) error = %v, want errors.Is(_, %v)", test.body, err, test.want)
			}
		})
	}
}

func boolPointer(value bool) *bool        { return &value }
func floatPointer(value float64) *float64 { return &value }
func intPointer(value int) *int           { return &value }
