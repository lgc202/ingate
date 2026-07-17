package runtime

import (
	"slices"
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
	if len(route.Config.Bindings) != 3 {
		t.Fatalf("len(Bindings) = %d, want 3", len(route.Config.Bindings))
	}
	for _, name := range []string{"gateway-binding", "route-binding", "primary-binding"} {
		if !slices.ContainsFunc(route.Config.Bindings, func(binding config.Binding) bool { return binding.Name == name }) {
			t.Fatalf("Bindings = %+v, want %s", route.Config.Bindings, name)
		}
	}
	for _, name := range []string{"x-gateway", "x-route", "x-risk-level"} {
		if !slices.Contains(route.HeaderNames, name) {
			t.Fatalf("HeaderNames = %v, want %s", route.HeaderNames, name)
		}
	}
	if slices.Contains(route.HeaderNames, "x-secondary") {
		t.Fatalf("HeaderNames = %v, must not include x-secondary", route.HeaderNames)
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
				Bindings: []config.Binding{
					{
						Name:   "gateway-binding",
						Target: config.Target{Kind: "Gateway", Name: "gw"},
						Policies: []config.Policy{
							{
								Name: "gateway-acl",
								Rules: []config.Rule{
									{
										Name:   "block-gateway",
										Action: config.ActionDeny,
										Conditions: []config.Condition{
											{Type: config.ConditionTypeHeader, Name: "x-gateway", Value: "deny"},
										},
									},
								},
							},
						},
					},
					{
						Name:   "route-binding",
						Target: config.Target{Kind: "Route", Name: "users"},
						Policies: []config.Policy{
							{
								Name: "route-acl",
								Rules: []config.Rule{
									{
										Name:   "block-route",
										Action: config.ActionDeny,
										Conditions: []config.Condition{
											{Type: config.ConditionTypeHeader, Name: "x-route", Value: "deny"},
										},
									},
								},
							},
						},
					},
					{
						Name:   "primary-binding",
						Target: config.Target{Kind: "Route", Name: "users", RuleName: "primary"},
						Policies: []config.Policy{
							{
								Name: "acl",
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
					},
					{
						Name:   "secondary-binding",
						Target: config.Target{Kind: "Route", Name: "users", RuleName: "secondary"},
						Policies: []config.Policy{
							{
								Name: "secondary-acl",
								Rules: []config.Rule{
									{
										Name:   "block-secondary",
										Action: config.ActionDeny,
										Conditions: []config.Condition{
											{Type: config.ConditionTypeHeader, Name: "x-secondary", Value: "deny"},
										},
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
