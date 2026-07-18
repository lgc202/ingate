package runtime

import (
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
	if got := route.Config.APIKey; got != "fallback-key" {
		t.Errorf("Runtime.Route(fallback rule) API key = %q, want %q", got, "fallback-key")
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

func routeConfig(ruleName, model, apiKey string) config.RouteConfig {
	return config.RouteConfig{
		GatewayName: "gateway-1",
		RouteName:   "route-1",
		RuleName:    ruleName,
		ConfigID:    ruleName + "-config",
		APIKey:      apiKey,
		Models: []config.ModelConfig{
			{Model: model, UpstreamModel: "gpt-4o-mini"},
		},
	}
}
