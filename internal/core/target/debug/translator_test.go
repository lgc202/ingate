package debug_test

import (
	"reflect"
	"testing"

	"github.com/lgc202/ingate/internal/core/ir"
	"github.com/lgc202/ingate/internal/core/target"
	"github.com/lgc202/ingate/internal/core/target/debug"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
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
		Upstreams: []ir.LogicalUpstream{
			{
				Name: "app",
				Endpoints: []ir.LogicalEndpoint{
					{Address: "10.0.0.10", Port: 8080},
				},
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
				Name: "app-quota",
				Mode: resource.RateLimitModeLocal,
				Rules: []ir.LogicalRateLimitRule{
					{
						Name: "consumer-minute",
						Key: []ir.LogicalRateLimitKeyPart{
							{Type: resource.RateLimitKeyTypeHeader, Name: "x-consumer-id"},
						},
						Limit: ir.LogicalRateLimitQuota{
							Requests:      100,
							WindowSeconds: 60,
						},
						Algorithm: resource.RateLimitAlgorithmFixedWindow,
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
		Upstreams: []debug.Upstream{
			{
				Name: "app",
				Endpoints: []debug.Endpoint{
					{Address: "10.0.0.10", Port: 8080},
				},
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
				Name: "app-quota",
				Mode: resource.RateLimitModeLocal,
				Rules: []debug.RateLimitRule{
					{
						Name: "consumer-minute",
						Key: []debug.RateLimitKeyPart{
							{Type: resource.RateLimitKeyTypeHeader, Name: "x-consumer-id"},
						},
						Limit: debug.RateLimitQuota{
							Requests:      100,
							WindowSeconds: 60,
						},
						Algorithm: resource.RateLimitAlgorithmFixedWindow,
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
