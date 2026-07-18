package policy

import (
	"encoding/json"
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
)

func TestRunnerRewritesSelectedModel(t *testing.T) {
	runner := NewRunner()
	models := map[string]config.ModelConfig{
		"assistant": {
			Model:         "assistant",
			UpstreamModel: "gpt-4o-mini",
		},
	}
	body := []byte(`{"model":"assistant","stream":true,"messages":[{"role":"user","content":"hello"}]}`)

	decision := runner.Apply(models, Request{Body: body})
	if decision.Rejection != nil {
		t.Fatalf("Runner.Apply(valid request).Rejection = %+v, want nil", decision.Rejection)
	}
	var got map[string]any
	if err := json.Unmarshal(decision.Mutation.Body, &got); err != nil {
		t.Fatalf("json.Unmarshal(Runner.Apply(valid request).Mutation.Body) error = %v", err)
	}
	if got["model"] != "gpt-4o-mini" {
		t.Errorf("Runner.Apply(valid request) model = %v, want %q", got["model"], "gpt-4o-mini")
	}
	if got["stream"] != true {
		t.Errorf("Runner.Apply(valid request) stream = %v, want true", got["stream"])
	}
}

func TestRunnerRejectsInvalidRequest(t *testing.T) {
	models := map[string]config.ModelConfig{
		"assistant": {Model: "assistant", UpstreamModel: "gpt-4o-mini"},
	}
	tests := []struct {
		name       string
		body       string
		statusCode int
		code       string
	}{
		{name: "empty body", statusCode: 400, code: "invalid_json"},
		{name: "non object", body: `[]`, statusCode: 400, code: "invalid_json"},
		{name: "invalid JSON", body: `{`, statusCode: 400, code: "invalid_json"},
		{name: "multiple JSON values", body: `{"model":"assistant"} {}`, statusCode: 400, code: "invalid_json"},
		{name: "missing model", body: `{"messages":[]}`, statusCode: 400, code: "model_required"},
		{name: "invalid model type", body: `{"model":1}`, statusCode: 400, code: "invalid_model"},
		{name: "unknown model", body: `{"model":"unknown"}`, statusCode: 404, code: "model_not_found"},
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
