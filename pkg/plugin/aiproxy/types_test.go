package aiproxy

import "testing"

func TestParsePluginConfig(t *testing.T) {
	data := []byte(`{
		"routes": [{
			"gatewayName": "gateway-1",
			"routeName": "route-1",
			"ruleName": "chat",
			"configID": "config-1",
			"apiKey": "secret",
			"models": [{
				"model": "assistant",
				"upstreamModel": "gpt-4o-mini"
			}]
		}]
	}`)

	cfg, err := ParsePluginConfig(data)
	if err != nil {
		t.Fatalf("ParsePluginConfig(valid config) error = %v, want nil", err)
	}
	if got := cfg.Routes[0].APIKey; got != "secret" {
		t.Errorf("ParsePluginConfig(valid config).Routes[0].APIKey = %q, want %q", got, "secret")
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
			name: "missing rule name",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","configID":"config-1","models":[{"model":"assistant","upstreamModel":"gpt-4o-mini"}]}]}`,
		},
		{
			name: "missing config ID",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","models":[{"model":"assistant","upstreamModel":"gpt-4o-mini"}]}]}`,
		},
		{
			name: "duplicate route rule",
			data: `{"routes": [
				{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-1","models":[{"model":"assistant","upstreamModel":"gpt-4o-mini"}]},
				{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-2","models":[{"model":"assistant-2","upstreamModel":"gpt-4o"}]}
			]}`,
		},
		{
			name: "duplicate model",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-1","models":[
				{"model":"assistant","upstreamModel":"gpt-4o-mini"},
				{"model":"assistant","upstreamModel":"gpt-4o"}
			]}]}`,
		},
		{
			name: "missing upstream model",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-1","models":[{"model":"assistant"}]}]}`,
		},
		{
			name: "API key contains newline",
			data: `{"routes": [{"gatewayName":"gateway-1","routeName":"route-1","ruleName":"chat","configID":"config-1","apiKey":"secret\r\ninjected","models":[{"model":"assistant","upstreamModel":"gpt-4o-mini"}]}]}`,
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
