package aiproxy

import "testing"

func TestParsePluginConfig(t *testing.T) {
	data := []byte(`{
		"routes": [{
			"gatewayName": "gateway-1",
			"routeName": "route-1",
			"ruleName": "chat",
			"configID": "config-1",
			"upstreams": [{
				"id": "openai",
				"protocol": "OpenAI",
				"cluster": "openai/ai/config",
				"basePath": "/v1",
				"apiKey": "secret",
				"apiKeyHeader": "authorization",
				"apiKeyPrefix": "Bearer "
			}],
			"models": [{
				"model": "assistant",
				"upstreamID": "openai",
				"upstreamModel": "gpt-4o-mini"
			}]
		}]
	}`)

	cfg, err := ParsePluginConfig(data)
	if err != nil {
		t.Fatalf("ParsePluginConfig(valid config) error = %v, want nil", err)
	}
	if got := cfg.Routes[0].Upstreams[0].APIKey; got != "secret" {
		t.Errorf("ParsePluginConfig(valid config).Routes[0].Upstreams[0].APIKey = %q, want %q", got, "secret")
	}
}

func TestParsePluginConfigAcceptsEmptyRoutes(t *testing.T) {
	cfg, err := ParsePluginConfig([]byte(`{"routes":[]}`))
	if err != nil {
		t.Fatalf("ParsePluginConfig(empty routes) error = %v, want nil", err)
	}
	if got := len(cfg.Routes); got != 0 {
		t.Errorf("ParsePluginConfig(empty routes) route count = %d, want 0", got)
	}
}

func TestParsePluginConfigRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "unknown field",
			data: `{"routes": [], "provider": "openai"}`,
		},
		{
			name: "upstream provider field",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-1","upstreams":[{"id":"upstream-1","provider":"openai","protocol":"OpenAI","cluster":"cluster","basePath":"/v1"}],"models":[` + modelConfigJSON("assistant", "upstream-1", "gpt-4o-mini") + `]}]}`,
		},
		{
			name: "missing rule name",
			data: routeConfigJSON(`"routeName":"route-1"`, modelConfigJSON("assistant", "upstream-1", "gpt-4o-mini")),
		},
		{
			name: "missing config ID",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","upstreams":[` + upstreamConfigJSON("upstream-1") + `],"models":[` + modelConfigJSON("assistant", "upstream-1", "gpt-4o-mini") + `]}]}`,
		},
		{
			name: "duplicate route rule",
			data: `{"routes": [
				{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-1","upstreams":[` + upstreamConfigJSON("upstream-1") + `],"models":[` + modelConfigJSON("assistant", "upstream-1", "gpt-4o-mini") + `]},
				{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-2","upstreams":[` + upstreamConfigJSON("upstream-2") + `],"models":[` + modelConfigJSON("assistant-2", "upstream-2", "gpt-4o") + `]}
			]}`,
		},
		{
			name: "duplicate model",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-1","upstreams":[` + upstreamConfigJSON("upstream-1") + `],"models":[
				` + modelConfigJSON("assistant", "upstream-1", "gpt-4o-mini") + `,
				` + modelConfigJSON("assistant", "upstream-1", "gpt-4o") + `
			]}]}`,
		},
		{
			name: "missing upstream model",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-1","upstreams":[` + upstreamConfigJSON("upstream-1") + `],"models":[{"model":"assistant","upstreamID":"upstream-1"}]}]}`,
		},
		{
			name: "API key contains newline",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-1","upstreams":[{"id":"upstream-1","protocol":"OpenAI","cluster":"cluster","basePath":"/v1","apiKey":"secret\r\ninjected","apiKeyHeader":"authorization","apiKeyPrefix":"Bearer "}],"models":[` + modelConfigJSON("assistant", "upstream-1", "gpt-4o-mini") + `]}]}`,
		},
		{
			name: "model upstream does not exist",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-1","upstreams":[` + upstreamConfigJSON("upstream-1") + `],"models":[` + modelConfigJSON("assistant", "missing", "gpt-4o-mini") + `]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParsePluginConfig([]byte(tt.data)); err == nil {
				t.Errorf("ParsePluginConfig(%s) error = nil, want error", tt.name)
			}
		})
	}
}

func routeConfigJSON(extra, model string) string {
	return `{"routes": [{"gatewayName":"gateway-1",` + extra + `,"configID":"config-1","upstreams":[` + upstreamConfigJSON("upstream-1") + `],"models":[` + model + `]}]}`
}

func upstreamConfigJSON(id string) string {
	return `{"id":"` + id + `","protocol":"OpenAI","cluster":"cluster","basePath":"/v1"}`
}

func modelConfigJSON(model, upstreamID, upstreamModel string) string {
	return `{"model":"` + model + `","upstreamID":"` + upstreamID + `","upstreamModel":"` + upstreamModel + `"}`
}
