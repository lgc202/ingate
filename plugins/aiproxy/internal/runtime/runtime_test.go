package runtime

import (
	"encoding/json"
	"strings"
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
	"github.com/lgc202/ingate/plugins/aiproxy/internal/policy"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
)

func TestRuntimeIndexesRouteRule(t *testing.T) {
	cfg := config.PluginConfig{Routes: []config.RouteConfig{
		routeConfig("primary", "assistant", "primary-key"),
		routeConfig("fallback", "assistant", "fallback-key"),
	}}
	runtime := Compile(cfg, policy.NewRunner())

	route, ok := runtime.Route(pluginruntime.RouteKey{
		GatewayName: "gateway-1",
		RouteName:   "route-1",
		RuleName:    "fallback",
		ConfigID:    "fallback-config",
	})
	if !ok {
		t.Fatal("Runtime.Route(fallback rule) ok = false, want true")
	}
	if got := route.Targets["fallback-target"].APIKey; got != "fallback-key" {
		t.Errorf("Runtime.Route(fallback rule) API key = %q, want %q", got, "fallback-key")
	}
}

func TestRuntimeAppliesProviderRequestPlan(t *testing.T) {
	route := Route{
		Config: config.RouteConfig{ConfigID: "route-config-1"},
		Targets: map[string]config.TargetConfig{
			"openai": {
				ID: "openai", Protocol: config.ProtocolOpenAI, Cluster: "cluster-openai",
				BasePath: "/v1", APIKey: "openai-key",
				APIKeyHeader: "authorization", APIKeyPrefix: "Bearer ",
			},
			"anthropic": {
				ID: "anthropic", Protocol: config.ProtocolAnthropic, Cluster: "cluster-anthropic",
				BasePath: "/v1", APIKey: "anthropic-key",
				APIKeyHeader: "x-api-key", Headers: []config.HeaderConfig{{Name: "anthropic-version", Value: "2023-06-01"}},
			},
			"gemini": {
				ID: "gemini", Protocol: config.ProtocolGemini, Cluster: "cluster-gemini",
				BasePath: "/v1beta", APIKey: "gemini-key",
				APIKeyHeader: "x-goog-api-key",
			},
		},
		Models: map[string]config.ModelConfig{
			"chat":   {Model: "chat", TargetID: "openai", UpstreamModel: "gpt-4o-mini"},
			"claude": {Model: "claude", TargetID: "anthropic", UpstreamModel: "claude-sonnet-4"},
			"gemini": {Model: "gemini", TargetID: "gemini", UpstreamModel: "gemini-2.5-flash"},
		},
	}
	runtime := Compile(config.PluginConfig{}, policy.NewRunner())
	tests := []struct {
		name          string
		model         string
		stream        bool
		wantCluster   string
		wantPath      string
		wantProtocol  config.Protocol
		wantBodyModel string
		wantHeader    string
	}{
		{name: "OpenAI", model: "chat", wantCluster: "cluster-openai", wantPath: "/v1/chat/completions", wantProtocol: config.ProtocolOpenAI, wantBodyModel: "gpt-4o-mini", wantHeader: "authorization"},
		{name: "Anthropic", model: "claude", wantCluster: "cluster-anthropic", wantPath: "/v1/messages", wantProtocol: config.ProtocolAnthropic, wantBodyModel: "claude-sonnet-4", wantHeader: "x-api-key"},
		{name: "Gemini stream", model: "gemini", stream: true, wantCluster: "cluster-gemini", wantPath: "/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse", wantProtocol: config.ProtocolGemini, wantHeader: "x-goog-api-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"model":"` + tt.model + `","messages":[{"role":"user","content":"hello"}]`
			if tt.stream {
				body += `,"stream":true`
			}
			body += `}`
			result := runtime.Apply(route, []byte(body))
			if result.Action.Kind != pluginruntime.ActionContinue {
				t.Fatalf("Runtime.Apply(%s) action = %#v, want continue", tt.name, result.Action)
			}
			if result.Mutation.Cluster != tt.wantCluster || result.Mutation.Path != tt.wantPath {
				t.Errorf("Runtime.Apply(%s) target = %q %q, want %q %q", tt.name, result.Mutation.Cluster, result.Mutation.Path, tt.wantCluster, tt.wantPath)
			}
			if result.Mutation.RouteConfigID != "route-config-1" {
				t.Errorf("Runtime.Apply(%s) route config ID = %q, want %q", tt.name, result.Mutation.RouteConfigID, "route-config-1")
			}
			if result.ResponsePlan == nil || result.ResponsePlan.Protocol != tt.wantProtocol || result.ResponsePlan.PublicModel != tt.model || result.ResponsePlan.Stream != tt.stream {
				t.Errorf("Runtime.Apply(%s) response plan = %#v, want protocol %q model %q stream %t", tt.name, result.ResponsePlan, tt.wantProtocol, tt.model, tt.stream)
			}
			if !containsHeader(result.Mutation.Headers, tt.wantHeader) {
				t.Errorf("Runtime.Apply(%s) headers = %#v, want %q", tt.name, result.Mutation.Headers, tt.wantHeader)
			}
			if tt.wantBodyModel != "" {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(result.Mutation.Body, &fields); err != nil {
					t.Fatalf("json.Unmarshal(Runtime.Apply(%s).Body) error = %v", tt.name, err)
				}
				if got := string(fields["model"]); got != `"`+tt.wantBodyModel+`"` {
					t.Errorf("Runtime.Apply(%s) body model = %s, want %q", tt.name, got, tt.wantBodyModel)
				}
			}
		})
	}
}

func TestRuntimeReturnsOpenAIErrorEnvelope(t *testing.T) {
	runtime := Compile(config.PluginConfig{}, policy.NewRunner())

	result := runtime.ValidateEndpoint("GET", "/v1/chat/completions")
	if result.Action.StatusCode != 405 {
		t.Fatalf("Runtime.ValidateEndpoint(GET).StatusCode = %d, want 405", result.Action.StatusCode)
	}
	if got := result.Action.Headers["content-type"]; got != "application/json" {
		t.Errorf("Runtime.ValidateEndpoint(GET) content-type = %q, want %q", got, "application/json")
	}
	if !strings.Contains(result.Action.Body, `"error"`) || !strings.Contains(result.Action.Body, `"method_not_allowed"`) {
		t.Errorf("Runtime.ValidateEndpoint(GET).Body = %q, want OpenAI-compatible error envelope", result.Action.Body)
	}
}

func TestRuntimeDoesNotAttributeProviderConversionErrorToModel(t *testing.T) {
	runtime := Compile(config.PluginConfig{}, policy.NewRunner())
	route := Route{
		Config: config.RouteConfig{ConfigID: "route-config-1"},
		Targets: map[string]config.TargetConfig{
			"anthropic": {
				ID: "anthropic", Protocol: config.ProtocolAnthropic, Cluster: "cluster-anthropic",
				BasePath: "/v1",
			},
		},
		Models: map[string]config.ModelConfig{
			"claude": {Model: "claude", TargetID: "anthropic", UpstreamModel: "claude-sonnet-4"},
		},
	}
	body := []byte(`{"model":"claude","messages":[{"role":"system","content":"answer briefly"}]}`)

	result := runtime.Apply(route, body)
	if result.Action.Kind != pluginruntime.ActionRespond || result.Action.StatusCode != 400 {
		t.Fatalf("Runtime.Apply(Anthropic system-only request) action = %#v, want 400 response", result.Action)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal([]byte(result.Action.Body), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(Runtime.Apply(Anthropic system-only request).Body) error = %v", err)
	}
	if envelope.Error.Param != nil {
		t.Errorf("Runtime.Apply(Anthropic system-only request) param = %q, want null", *envelope.Error.Param)
	}
}

func routeConfig(ruleName, model, apiKey string) config.RouteConfig {
	targetID := ruleName + "-target"
	return config.RouteConfig{
		GatewayName: "gateway-1",
		RouteName:   "route-1",
		RuleName:    ruleName,
		ConfigID:    ruleName + "-config",
		Targets: []config.TargetConfig{{
			ID: targetID, Provider: "openai", Protocol: config.ProtocolOpenAI,
			Cluster: "cluster-" + ruleName, BasePath: "/v1",
			APIKey: apiKey, APIKeyHeader: "authorization", APIKeyPrefix: "Bearer ",
		}},
		Models: []config.ModelConfig{
			{Model: model, TargetID: targetID, UpstreamModel: "gpt-4o-mini"},
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
