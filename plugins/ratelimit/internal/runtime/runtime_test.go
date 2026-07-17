package runtime

import (
	"errors"
	"slices"
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
)

func TestCompileBuildsRouteIndexAndHeaderPlan(t *testing.T) {
	runtime := Compile(pluginConfig(), policy.NewMemoryRunner())

	route, ok := runtime.Route(pluginruntime.RouteKey{GatewayName: "gw", RouteName: "users", RuleName: "primary"})
	if !ok {
		t.Fatal("Route() ok = false, want true")
	}
	if route.Config.RouteName != "users" {
		t.Fatalf("route name = %q, want users", route.Config.RouteName)
	}
	if !slices.Contains(route.HeaderNames, "x-tenant") {
		t.Fatalf("HeaderNames = %v, want x-tenant", route.HeaderNames)
	}
}

func TestApplyReturnsRespondActionWhenLocalQuotaExceeded(t *testing.T) {
	runtime := Compile(pluginConfig(), policy.NewMemoryRunner())
	route, ok := runtime.Route(pluginruntime.RouteKey{GatewayName: "gw", RouteName: "users", RuleName: "primary"})
	if !ok {
		t.Fatal("Route() ok = false, want true")
	}
	req := policy.Request{
		GatewayName: "gw",
		RouteName:   "users",
		RuleName:    "primary",
		Headers:     map[string]string{"x-tenant": "acme"},
	}

	first := runtime.Apply(route, req)
	if first.Action.Kind != pluginruntime.ActionContinue {
		t.Fatalf("first action = %q, want Continue", first.Action.Kind)
	}
	second := runtime.Apply(route, req)
	if second.Action.Kind != pluginruntime.ActionRespond {
		t.Fatalf("second action = %q, want Respond", second.Action.Kind)
	}
	if second.Action.StatusCode != 429 {
		t.Fatalf("StatusCode = %d, want 429", second.Action.StatusCode)
	}
}

func TestCompleteGlobalChecksAppliesFailClosePolicy(t *testing.T) {
	runtime := Compile(pluginConfig(), policy.NewMemoryRunner())
	result := runtime.CompleteGlobalChecks([]policy.GlobalCheck{
		{
			Policy: config.Policy{
				Name:          "global",
				FailurePolicy: config.FailurePolicyFailClose,
			},
			Rule: config.Rule{Name: "tenant"},
			Key:  "Header=acme",
		},
	}, []policy.GlobalOutcome{{Err: errors.New("redis unavailable")}})

	if result.Action.Kind != pluginruntime.ActionRespond {
		t.Fatalf("action = %q, want Respond", result.Action.Kind)
	}
	if result.Action.StatusCode != 429 {
		t.Fatalf("StatusCode = %d, want 429", result.Action.StatusCode)
	}
}

func pluginConfig() config.PluginConfig {
	return config.PluginConfig{
		Routes: []config.RouteConfig{
			{
				GatewayName: "gw",
				RouteName:   "users",
				Bindings: []config.Binding{
					{
						Name:   "binding",
						Target: config.Target{Kind: "Route", Name: "users", RuleName: "primary"},
						Policies: []config.Policy{
							{
								Name: "local",
								Mode: config.ModeLocal,
								Rules: []config.Rule{
									{
										Name: "tenant",
										Key: []config.KeyPart{
											{Type: config.KeyTypeHeader, Name: "x-tenant"},
										},
										Limit: config.Quota{Requests: 1, WindowSeconds: 60},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
