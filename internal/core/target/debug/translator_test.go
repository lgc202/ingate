package debug_test

import (
	"reflect"
	"testing"

	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/resource"
	"github.com/lgc202/ingate-next/internal/core/target"
	"github.com/lgc202/ingate-next/internal/core/target/debug"
)

func TestTranslatorImplementsTargetTranslator(t *testing.T) {
	var translator target.Translator = debug.Translator{}
	if translator.Target() != "debug" {
		t.Fatalf("Target() = %q, want debug", translator.Target())
	}
}

func TestTranslatorTranslate(t *testing.T) {
	logical := ir.LogicalGateway{
		Name: "public",
		Listeners: []ir.LogicalListener{
			{Name: "http", Protocol: "HTTP", Port: 80, Hostname: "example.com"},
		},
		Routes: []ir.LogicalRoute{
			{
				Name:      "app",
				Hostnames: []string{"example.com"},
				Rules: []ir.LogicalRouteRule{
					{
						PathPrefix:    "/app",
						Methods:       []string{"GET", "POST"},
						TimeoutMillis: 3000,
						Headers: []ir.LogicalHeaderMatch{
							{Name: "x-tenant", Value: "acme"},
						},
						Upstreams: []ir.LogicalUpstreamRef{
							{Name: "app", Weight: 100},
						},
					},
				},
			},
		},
		AIRoutes: []ir.LogicalAIRoute{
			{
				Name:       "chat",
				PathPrefix: "/v1/chat/completions",
				Model:      "gpt-4.1-mini",
				Providers: []ir.LogicalAIProviderRef{
					{Name: "openai", Weight: 100},
				},
			},
		},
		Upstreams: []ir.LogicalUpstream{
			{
				Name: "app",
				Endpoints: []ir.LogicalEndpoint{
					{Address: "10.0.0.10", Port: 8080},
				},
			},
		},
		AIProviders: []ir.LogicalAIProvider{
			{
				Name:     "openai",
				Type:     resource.AIProviderTypeOpenAICompatible,
				Endpoint: "https://api.openai.com/v1",
				Models:   []string{"gpt-4.1-mini"},
			},
		},
		Plugins: []ir.LogicalPlugin{
			{
				Name:     "audit-log",
				Runtime:  resource.PluginRuntimeExternal,
				Version:  "v1",
				Endpoint: "dns:///audit-plugin:9000",
			},
		},
		AuthPolicies: []ir.LogicalAuthPolicy{
			{
				Name: "required",
				Type: resource.AuthTypeAPIKey,
				APIKey: ir.LogicalAPIKeyAuth{
					Header: "x-api-key",
				},
			},
		},
		RateLimitPolicies: []ir.LogicalRateLimitPolicy{
			{
				Name:          "app-quota",
				Requests:      100,
				WindowSeconds: 60,
				KeyBy:         resource.RateLimitKeyHeader,
				Header:        "x-consumer-id",
			},
		},
		PluginBindings: []ir.LogicalPluginBinding{
			{
				Name: "app-audit",
				Target: ir.LogicalPluginTarget{
					Kind: resource.KindRoute,
					Name: "app",
				},
				Plugins: []ir.LogicalPluginRef{
					{
						Name: "audit-log",
						Config: map[string]any{
							"mode": "audit",
						},
					},
				},
			},
		},
		PolicyBindings: []ir.LogicalPolicyBinding{
			{
				Name: "app-auth",
				Target: ir.LogicalPolicyTarget{
					Kind: resource.KindRoute,
					Name: "app",
				},
				Policies: []ir.LogicalPolicyRef{
					{Kind: resource.KindAuthPolicy, Name: "required"},
					{Kind: resource.KindRateLimitPolicy, Name: "app-quota"},
				},
			},
		},
	}

	snapshot, err := (debug.Translator{}).Translate(logical)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if snapshot.Target != "debug" {
		t.Fatalf("Target = %q, want debug", snapshot.Target)
	}
	if snapshot.Gateway != "public" {
		t.Fatalf("Gateway = %q, want public", snapshot.Gateway)
	}
	if snapshot.Version == "" {
		t.Fatal("Version is empty")
	}

	got, ok := snapshot.Config.(debug.Config)
	if !ok {
		t.Fatalf("Config type = %T, want debug.Config", snapshot.Config)
	}

	want := debug.Config{
		Listeners: []debug.Listener{
			{Name: "http", Protocol: "HTTP", Port: 80, Hostname: "example.com"},
		},
		Routes: []debug.Route{
			{
				Name:      "app",
				Hostnames: []string{"example.com"},
				Rules: []debug.RouteRule{
					{
						PathPrefix:    "/app",
						Methods:       []string{"GET", "POST"},
						TimeoutMillis: 3000,
						Headers: []debug.HeaderMatch{
							{Name: "x-tenant", Value: "acme"},
						},
						Upstreams: []debug.UpstreamRef{
							{Name: "app", Weight: 100},
						},
					},
				},
			},
		},
		AIRoutes: []debug.AIRoute{
			{
				Name:       "chat",
				PathPrefix: "/v1/chat/completions",
				Model:      "gpt-4.1-mini",
				Providers: []debug.AIProviderRef{
					{Name: "openai", Weight: 100},
				},
			},
		},
		Upstreams: []debug.Upstream{
			{
				Name: "app",
				Endpoints: []debug.Endpoint{
					{Address: "10.0.0.10", Port: 8080},
				},
			},
		},
		AIProviders: []debug.AIProvider{
			{
				Name:     "openai",
				Type:     resource.AIProviderTypeOpenAICompatible,
				Endpoint: "https://api.openai.com/v1",
				Models:   []string{"gpt-4.1-mini"},
			},
		},
		Plugins: []debug.Plugin{
			{
				Name:     "audit-log",
				Runtime:  resource.PluginRuntimeExternal,
				Version:  "v1",
				Endpoint: "dns:///audit-plugin:9000",
			},
		},
		AuthPolicies: []debug.AuthPolicy{
			{
				Name: "required",
				Type: resource.AuthTypeAPIKey,
				APIKey: debug.APIKeyAuth{
					Header: "x-api-key",
				},
			},
		},
		RateLimitPolicies: []debug.RateLimitPolicy{
			{
				Name:          "app-quota",
				Requests:      100,
				WindowSeconds: 60,
				KeyBy:         resource.RateLimitKeyHeader,
				Header:        "x-consumer-id",
			},
		},
		PluginBindings: []debug.PluginBinding{
			{
				Name: "app-audit",
				Target: debug.PluginTarget{
					Kind: resource.KindRoute,
					Name: "app",
				},
				Plugins: []debug.PluginRef{
					{
						Name: "audit-log",
						Config: map[string]any{
							"mode": "audit",
						},
					},
				},
			},
		},
		PolicyBindings: []debug.PolicyBinding{
			{
				Name: "app-auth",
				Target: debug.PolicyTarget{
					Kind: resource.KindRoute,
					Name: "app",
				},
				Policies: []debug.PolicyRef{
					{Kind: resource.KindAuthPolicy, Name: "required"},
					{Kind: resource.KindRateLimitPolicy, Name: "app-quota"},
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config = %#v, want %#v", got, want)
	}
}
