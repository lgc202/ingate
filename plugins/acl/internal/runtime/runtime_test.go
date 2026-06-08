package runtime

import (
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/acl"
	"github.com/lgc202/ingate/plugins/acl/internal/policy"
	pluginruntime "github.com/lgc202/ingate/plugins/internal/runtime"
)

func TestCompileBuildsRouteIndex(t *testing.T) {
	runtime := Compile(pluginConfig(), policy.NewRunner())

	route, ok := runtime.Route(pluginruntime.RouteKey{GatewayName: "gw", RouteName: "users", RuleName: "primary"})
	if !ok {
		t.Fatal("Route() ok = false, want true")
	}
	if route.Config.RouteName != "users" {
		t.Fatalf("route name = %q, want users", route.Config.RouteName)
	}
	if len(route.HeaderNames) != 1 || route.HeaderNames[0] != "x-risk-level" {
		t.Fatalf("HeaderNames = %v, want [x-risk-level]", route.HeaderNames)
	}
}

func TestApplyReturnsRespondActionWhenRequestDenied(t *testing.T) {
	runtime := Compile(pluginConfig(), policy.NewRunner())
	route, ok := runtime.Route(pluginruntime.RouteKey{GatewayName: "gw", RouteName: "users", RuleName: "primary"})
	if !ok {
		t.Fatal("Route() ok = false, want true")
	}

	action := runtime.Apply(route, policy.Request{Headers: map[string]string{"x-risk-level": "high"}})
	if action.Kind != pluginruntime.ActionRespond {
		t.Fatalf("Kind = %q, want Respond", action.Kind)
	}
	if action.StatusCode != 403 {
		t.Fatalf("StatusCode = %d, want 403", action.StatusCode)
	}
}

func pluginConfig() config.PluginConfig {
	return config.PluginConfig{
		Routes: []config.RouteConfig{
			{
				GatewayName: "gw",
				RouteName:   "users",
				RuleName:    "primary",
				Rules: []config.Rule{
					{
						Name:   "block-risk",
						Action: config.ActionDeny,
						Conditions: []config.Condition{
							{Type: config.ConditionTypeHeader, Name: "x-risk-level", Value: "high"},
						},
					},
				},
			},
		},
	}
}
