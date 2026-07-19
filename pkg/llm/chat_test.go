package llm

import (
	"errors"
	"reflect"
	"testing"
)

func TestDecodeChatRequest(t *testing.T) {
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

	got, err := DecodeChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeChatRequest(%s) returned error: %v", body, err)
	}
	want := ChatRequest{
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
		t.Errorf("DecodeChatRequest(%s) = %#v, want %#v", body, got, want)
	}
	if !got.Streaming() {
		t.Errorf("ChatRequest.Streaming() = false, want true")
	}
	if err := got.ValidateSupported(); err != nil {
		t.Errorf("ChatRequest.ValidateSupported() returned error: %v", err)
	}
}

func TestChatRequestValidateSupported_RejectsUnknownFields(t *testing.T) {
	body := []byte(`{
		"model":"m",
		"messages":[{"role":"user","content":"hello","name":"client"}],
		"frequency_penalty":0.2
	}`)
	request, err := DecodeChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeChatRequest(%s) returned error: %v", body, err)
	}
	if err := request.ValidateSupported(); !errors.Is(err, ErrUnsupportedFeature) {
		t.Errorf("ChatRequest.ValidateSupported() error = %v, want errors.Is(_, ErrUnsupportedFeature)", err)
	}
}

func TestChatRequestValidateSupported_RejectsLateSystemMessage(t *testing.T) {
	body := []byte(`{
		"model":"m",
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"system","content":"answer briefly"}
		]
	}`)
	request, err := DecodeChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeChatRequest(%s) returned error: %v", body, err)
	}
	if err := request.ValidateSupported(); !errors.Is(err, ErrUnsupportedFeature) {
		t.Errorf("ChatRequest.ValidateSupported() error = %v, want errors.Is(_, ErrUnsupportedFeature)", err)
	}
}

func TestDecodeChatRequest_RejectsInvalidOrUnsupportedInput(t *testing.T) {
	tests := []struct {
		name string
		body string
		want error
	}{
		{name: "array body", body: `[]`, want: ErrInvalidRequest},
		{name: "missing model", body: `{"messages":[{"role":"user","content":"x"}]}`, want: ErrInvalidRequest},
		{name: "null content", body: `{"model":"m","messages":[{"role":"user","content":null}]}`, want: ErrInvalidRequest},
		{name: "multimodal content", body: `{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"x"}]}]}`, want: ErrUnsupportedFeature},
		{name: "tool role", body: `{"model":"m","messages":[{"role":"tool","content":"x"}]}`, want: ErrUnsupportedFeature},
		{name: "tools", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"tools":[]}`, want: ErrUnsupportedFeature},
		{name: "function call", body: `{"model":"m","messages":[{"role":"assistant","content":"","function_call":{"name":"x"}}]}`, want: ErrUnsupportedFeature},
		{name: "null stream", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"stream":null}`, want: ErrInvalidRequest},
		{name: "temperature range", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"temperature":2.1}`, want: ErrInvalidRequest},
		{name: "top p type", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"top_p":"1"}`, want: ErrInvalidRequest},
		{name: "fractional max tokens", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"max_tokens":1.5}`, want: ErrInvalidRequest},
		{name: "zero max tokens", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"max_tokens":0}`, want: ErrInvalidRequest},
		{name: "invalid stop", body: `{"model":"m","messages":[{"role":"user","content":"x"}],"stop":{"value":"x"}}`, want: ErrInvalidRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeChatRequest([]byte(test.body))
			if !errors.Is(err, test.want) {
				t.Errorf("DecodeChatRequest(%s) error = %v, want errors.Is(_, %v)", test.body, err, test.want)
			}
		})
	}
}

func boolPointer(value bool) *bool        { return &value }
func floatPointer(value float64) *float64 { return &value }
func intPointer(value int) *int           { return &value }
