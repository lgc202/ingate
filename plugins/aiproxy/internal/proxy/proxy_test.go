package proxy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/openai"
	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
)

func TestProxyIndexesRouteRule(t *testing.T) {
	cfg := config.PluginConfig{Routes: []config.RouteConfig{
		routeConfig("primary", "assistant", "primary-key"),
		routeConfig("fallback", "assistant", "fallback-key"),
	}}
	modelProxy := New(cfg)

	route, ok := modelProxy.Route(RouteKey{
		GatewayName: "gateway-1",
		RouteName:   "route-1",
		RuleName:    "fallback",
		ConfigID:    "fallback-config",
	})
	if !ok {
		t.Fatal("Proxy.Route(fallback rule) ok = false, want true")
	}
	if got := route.Upstreams["fallback-upstream"].APIKey; got != "fallback-key" {
		t.Errorf("Proxy.Route(fallback rule) API key = %q, want %q", got, "fallback-key")
	}
}

func TestProxyPreparesProviderRequest(t *testing.T) {
	route := Route{
		ConfigID: "route-config-1",
		Upstreams: map[string]config.UpstreamConfig{
			"openai": {
				ID: "openai", Protocol: llm.ProtocolOpenAIChatCompletions, Cluster: "cluster-openai",
				BasePath: "/v1", APIKey: "openai-key",
				APIKeyHeader: "authorization", APIKeyPrefix: "Bearer ",
			},
			"anthropic": {
				ID: "anthropic", Protocol: llm.ProtocolAnthropicMessages, Cluster: "cluster-anthropic",
				BasePath: "/v1", APIKey: "anthropic-key",
				APIKeyHeader: "x-api-key", Headers: []config.HeaderConfig{{Name: "anthropic-version", Value: "2023-06-01"}},
			},
			"gemini": {
				ID: "gemini", Protocol: llm.ProtocolGeminiGenerateContent, Cluster: "cluster-gemini",
				BasePath: "/v1beta", APIKey: "gemini-key",
				APIKeyHeader: "x-goog-api-key",
			},
		},
		Models: map[string]config.ModelConfig{
			"chat":   {Model: "chat", UpstreamID: "openai", UpstreamModel: "gpt-4o-mini"},
			"claude": {Model: "claude", UpstreamID: "anthropic", UpstreamModel: "claude-sonnet-4"},
			"gemini": {Model: "gemini", UpstreamID: "gemini", UpstreamModel: "gemini-2.5-flash"},
		},
	}
	modelProxy := New(config.PluginConfig{})
	tests := []struct {
		name          string
		model         string
		stream        bool
		wantCluster   string
		wantPath      string
		wantProtocol  llm.Protocol
		wantBodyModel string
		wantHeader    string
	}{
		{name: "OpenAI", model: "chat", wantCluster: "cluster-openai", wantPath: "/v1/chat/completions", wantProtocol: llm.ProtocolOpenAIChatCompletions, wantBodyModel: "gpt-4o-mini", wantHeader: "authorization"},
		{name: "Anthropic", model: "claude", wantCluster: "cluster-anthropic", wantPath: "/v1/messages", wantProtocol: llm.ProtocolAnthropicMessages, wantBodyModel: "claude-sonnet-4", wantHeader: "x-api-key"},
		{name: "Gemini stream", model: "gemini", stream: true, wantCluster: "cluster-gemini", wantPath: "/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse", wantProtocol: llm.ProtocolGeminiGenerateContent, wantHeader: "x-goog-api-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"model":"` + tt.model + `","messages":[{"role":"user","content":"hello"}]`
			if tt.stream {
				body += `,"stream":true`
			}
			body += `}`
			request, response := modelProxy.PrepareRequest(route, []byte(body))
			if response != nil {
				t.Fatalf("Proxy.PrepareRequest(%s) response = %#v, want nil", tt.name, response)
			}
			if request.Cluster != tt.wantCluster || request.Path != tt.wantPath {
				t.Errorf("Proxy.PrepareRequest(%s) upstream = %q %q, want %q %q", tt.name, request.Cluster, request.Path, tt.wantCluster, tt.wantPath)
			}
			if request.RouteConfigID != "route-config-1" {
				t.Errorf("Proxy.PrepareRequest(%s) route config ID = %q, want %q", tt.name, request.RouteConfigID, "route-config-1")
			}
			if request.Response.Protocol != tt.wantProtocol || request.Response.PublicModel != tt.model || request.Response.Stream != tt.stream {
				t.Errorf("Proxy.PrepareRequest(%s) response transform = %#v, want protocol %q model %q stream %t", tt.name, request.Response, tt.wantProtocol, tt.model, tt.stream)
			}
			if !containsHeader(request.Headers, tt.wantHeader) {
				t.Errorf("Proxy.PrepareRequest(%s) headers = %#v, want %q", tt.name, request.Headers, tt.wantHeader)
			}
			if tt.wantBodyModel != "" {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(request.Body, &fields); err != nil {
					t.Fatalf("json.Unmarshal(Proxy.PrepareRequest(%s).Body) error = %v", tt.name, err)
				}
				if got := string(fields["model"]); got != `"`+tt.wantBodyModel+`"` {
					t.Errorf("Proxy.PrepareRequest(%s) body model = %s, want %q", tt.name, got, tt.wantBodyModel)
				}
			}
		})
	}
}

func TestProxyReturnsOpenAIErrorEnvelope(t *testing.T) {
	modelProxy := New(config.PluginConfig{})

	response := modelProxy.ValidateEndpoint("GET", "/v1/chat/completions")
	if response == nil {
		t.Fatal("Proxy.ValidateEndpoint(GET) response = nil, want method rejection")
	}
	if response.StatusCode != 405 {
		t.Fatalf("Proxy.ValidateEndpoint(GET).StatusCode = %d, want 405", response.StatusCode)
	}
	if got := response.Headers["content-type"]; got != "application/json" {
		t.Errorf("Proxy.ValidateEndpoint(GET) content-type = %q, want %q", got, "application/json")
	}
	if !strings.Contains(response.Body, `"error"`) || !strings.Contains(response.Body, `"method_not_allowed"`) {
		t.Errorf("Proxy.ValidateEndpoint(GET).Body = %q, want OpenAI-compatible error envelope", response.Body)
	}
}

func TestProxyDoesNotAttributeProviderConversionErrorToModel(t *testing.T) {
	modelProxy := New(config.PluginConfig{})
	route := Route{
		ConfigID: "route-config-1",
		Upstreams: map[string]config.UpstreamConfig{
			"anthropic": {
				ID: "anthropic", Protocol: llm.ProtocolAnthropicMessages, Cluster: "cluster-anthropic",
				BasePath: "/v1",
			},
		},
		Models: map[string]config.ModelConfig{
			"claude": {Model: "claude", UpstreamID: "anthropic", UpstreamModel: "claude-sonnet-4"},
		},
	}
	body := []byte(`{"model":"claude","messages":[{"role":"system","content":"answer briefly"}]}`)

	_, response := modelProxy.PrepareRequest(route, body)
	if response == nil || response.StatusCode != 400 {
		t.Fatalf("Proxy.PrepareRequest(Anthropic system-only request) response = %#v, want status 400", response)
	}
	var envelope openai.ErrorEnvelope
	if err := json.Unmarshal([]byte(response.Body), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(Proxy.PrepareRequest(Anthropic system-only request).Body) error = %v", err)
	}
	if envelope.Error.Param != nil {
		t.Errorf("Proxy.PrepareRequest(Anthropic system-only request) param = %q, want null", *envelope.Error.Param)
	}
}

func routeConfig(ruleName, model, apiKey string) config.RouteConfig {
	upstreamID := ruleName + "-upstream"
	return config.RouteConfig{
		GatewayName: "gateway-1",
		RouteName:   "route-1",
		RuleName:    ruleName,
		ConfigID:    ruleName + "-config",
		Upstreams: []config.UpstreamConfig{{
			ID: upstreamID, Provider: "openai", Protocol: llm.ProtocolOpenAIChatCompletions,
			Cluster: "cluster-" + ruleName, BasePath: "/v1",
			APIKey: apiKey, APIKeyHeader: "authorization", APIKeyPrefix: "Bearer ",
		}},
		Models: []config.ModelConfig{
			{Model: model, UpstreamID: upstreamID, UpstreamModel: "gpt-4o-mini"},
		},
	}
}

func containsHeader(headers []config.HeaderConfig, name string) bool {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return true
		}
	}
	return false
}
