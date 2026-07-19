package policy

import (
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
)

func TestRunnerSelectsModelAndStreamMode(t *testing.T) {
	runner := NewRunner()
	models := map[string]config.ModelConfig{
		"assistant": {
			Model:         "assistant",
			TargetID:      "openai-account",
			UpstreamModel: "gpt-4o-mini",
		},
	}
	body := []byte(`{"model":"assistant","stream":true,"messages":[{"role":"user","content":"hello"}]}`)

	decision := runner.Apply(models, Request{Body: body})
	if decision.Rejection != nil {
		t.Fatalf("Runner.Apply(valid request).Rejection = %+v, want nil", decision.Rejection)
	}
	if decision.Selection == nil {
		t.Fatal("Runner.Apply(valid request).Selection = nil, want selection")
	}
	if got, want := decision.Selection.Model.TargetID, "openai-account"; got != want {
		t.Errorf("Runner.Apply(valid request) target = %q, want %q", got, want)
	}
	if !decision.Selection.Stream {
		t.Error("Runner.Apply(valid request) stream = false, want true")
	}
}

func TestRunnerRejectsInvalidRequest(t *testing.T) {
	models := map[string]config.ModelConfig{
		"assistant": {Model: "assistant", TargetID: "openai-account", UpstreamModel: "gpt-4o-mini"},
	}
	tests := []struct {
		name       string
		body       string
		statusCode int
		code       string
	}{
		{name: "empty body", statusCode: 400, code: "invalid_request"},
		{name: "non object", body: `[]`, statusCode: 400, code: "invalid_request"},
		{name: "invalid JSON", body: `{`, statusCode: 400, code: "invalid_request"},
		{name: "multiple JSON values", body: `{"model":"assistant"} {}`, statusCode: 400, code: "invalid_request"},
		{name: "missing model", body: `{"messages":[{"role":"user","content":"hi"}]}`, statusCode: 400, code: "invalid_request"},
		{name: "invalid model type", body: `{"model":1,"messages":[{"role":"user","content":"hi"}]}`, statusCode: 400, code: "invalid_request"},
		{name: "unknown model", body: `{"model":"unknown","messages":[{"role":"user","content":"hi"}]}`, statusCode: 404, code: "model_not_found"},
		{name: "unknown model with unsupported field", body: `{"model":"unknown","messages":[{"role":"user","content":"hi"}],"frequency_penalty":0.2}`, statusCode: 400, code: "unsupported_feature"},
		{name: "tools unsupported", body: `{"model":"assistant","messages":[{"role":"user","content":"hi"}],"tools":[]}`, statusCode: 400, code: "unsupported_feature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := NewRunner().Apply(models, Request{Body: []byte(tt.body)})
			if decision.Rejection == nil {
				t.Fatalf("Runner.Apply(%s).Rejection = nil, want rejection", tt.name)
			}
			if decision.Rejection.StatusCode != tt.statusCode || decision.Rejection.Code != tt.code {
				t.Errorf("Runner.Apply(%s).Rejection = %+v, want status %d and code %q", tt.name, decision.Rejection, tt.statusCode, tt.code)
			}
		})
	}
}

func TestRunnerValidatesSupportedEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		request    Request
		statusCode int
	}{
		{name: "supported", request: Request{Method: "POST", Path: "/v1/chat/completions?trace=true"}},
		{name: "unsupported method", request: Request{Method: "GET", Path: "/v1/chat/completions"}, statusCode: 405},
		{name: "unsupported path", request: Request{Method: "POST", Path: "/v1/responses"}, statusCode: 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rejection := NewRunner().ValidateEndpoint(tt.request)
			if tt.statusCode == 0 {
				if rejection != nil {
					t.Errorf("Runner.ValidateEndpoint(%s) = %+v, want nil", tt.name, rejection)
				}
				return
			}
			if rejection == nil || rejection.StatusCode != tt.statusCode {
				t.Errorf("Runner.ValidateEndpoint(%s) = %+v, want status %d", tt.name, rejection, tt.statusCode)
			}
		})
	}
}
