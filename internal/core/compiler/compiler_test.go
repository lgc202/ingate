package compiler_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lgc202/ingate-next/internal/core/compiler"
	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/resource"
)

func TestCompilerCompileGateway(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{
				Metadata: resource.Metadata{Name: "public"},
				Spec: resource.GatewaySpec{
					Listeners: []resource.Listener{
						{Name: "http", Protocol: "HTTP", Port: 80, Hostname: "example.com"},
					},
				},
			},
		},
		Routes: []resource.Route{
			{
				Metadata: resource.Metadata{Name: "app"},
				Spec: resource.RouteSpec{
					ParentRefs: []string{"public"},
					Hostnames:  []string{"example.com"},
					Rules: []resource.RouteRule{
						{
							PathPrefix:    "/app",
							Methods:       []string{"GET", "POST"},
							TimeoutMillis: 3000,
							Headers: []resource.HeaderMatch{
								{Name: "x-tenant", Value: "acme"},
							},
							UpstreamRefs: []resource.UpstreamRef{
								{Name: "app", Weight: 100},
							},
						},
					},
				},
			},
		},
		AIRoutes: []resource.AIRoute{
			{
				Metadata: resource.Metadata{Name: "chat"},
				Spec: resource.AIRouteSpec{
					ParentRefs: []string{"public"},
					PathPrefix: "/v1/chat/completions",
					Model:      "gpt-4.1-mini",
					ProviderRefs: []resource.AIProviderRef{
						{Name: "openai", Weight: 100},
					},
				},
			},
		},
		Upstreams: []resource.Upstream{
			{
				Metadata: resource.Metadata{Name: "app"},
				Spec: resource.UpstreamSpec{
					Endpoints: []resource.Endpoint{
						{Address: "10.0.0.10", Port: 8080},
					},
				},
			},
		},
		AIProviders: []resource.AIProvider{
			{
				Metadata: resource.Metadata{Name: "openai"},
				Spec: resource.AIProviderSpec{
					Type:     resource.AIProviderTypeOpenAICompatible,
					Endpoint: "https://api.openai.com/v1",
					Models:   []string{"gpt-4.1-mini"},
				},
			},
		},
		Plugins: []resource.Plugin{
			{
				Metadata: resource.Metadata{Name: "audit-log"},
				Spec: resource.PluginSpec{
					Runtime:  resource.PluginRuntimeExternal,
					Version:  "v1",
					Endpoint: "dns:///audit-plugin:9000",
				},
			},
		},
		AuthPolicies: []resource.AuthPolicy{
			{
				Metadata: resource.Metadata{Name: "required"},
				Spec: resource.AuthPolicySpec{
					Type: resource.AuthTypeAPIKey,
					APIKey: resource.APIKeyAuth{
						Header: "x-api-key",
					},
				},
			},
		},
		RateLimitPolicies: []resource.RateLimitPolicy{
			{
				Metadata: resource.Metadata{Name: "app-quota"},
				Spec: resource.RateLimitPolicySpec{
					Requests:      100,
					WindowSeconds: 60,
					KeyBy:         resource.RateLimitKeyHeader,
					Header:        "x-consumer-id",
				},
			},
		},
		PluginBindings: []resource.PluginBinding{
			{
				Metadata: resource.Metadata{Name: "app-audit"},
				Spec: resource.PluginBindingSpec{
					TargetRef: resource.PluginTargetRef{
						Kind: resource.KindRoute,
						Name: "app",
					},
					Plugins: []resource.PluginRef{
						{
							Name: "audit-log",
							Config: map[string]any{
								"mode": "audit",
							},
						},
					},
				},
			},
		},
		PolicyBindings: []resource.PolicyBinding{
			{
				Metadata: resource.Metadata{Name: "app-auth"},
				Spec: resource.PolicyBindingSpec{
					TargetRef: resource.PolicyTargetRef{
						Kind: resource.KindRoute,
						Name: "app",
					},
					Policies: []resource.PolicyRef{
						{Kind: resource.KindAuthPolicy, Name: "required"},
						{Kind: resource.KindRateLimitPolicy, Name: "app-quota"},
					},
				},
			},
		},
	}

	got, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err != nil {
		t.Fatalf("CompileGateway() error = %v", err)
	}

	want := ir.LogicalGateway{
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

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CompileGateway() = %#v, want %#v", got, want)
	}
}

func TestCompilerCompileGatewayMissingAIProvider(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
		},
		AIRoutes: []resource.AIRoute{
			{
				Metadata: resource.Metadata{Name: "chat"},
				Spec: resource.AIRouteSpec{
					ParentRefs: []string{"public"},
					PathPrefix: "/v1/chat/completions",
					Model:      "gpt-4.1-mini",
					ProviderRefs: []resource.AIProviderRef{
						{Name: "missing", Weight: 100},
					},
				},
			},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `ai route "chat" references ai provider "missing"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayMissingPluginRef(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
		},
		Routes: []resource.Route{
			{
				Metadata: resource.Metadata{Name: "app"},
				Spec: resource.RouteSpec{
					ParentRefs: []string{"public"},
				},
			},
		},
		PluginBindings: []resource.PluginBinding{
			{
				Metadata: resource.Metadata{Name: "app-audit"},
				Spec: resource.PluginBindingSpec{
					TargetRef: resource.PluginTargetRef{
						Kind: resource.KindRoute,
						Name: "app",
					},
					Plugins: []resource.PluginRef{
						{Name: "missing"},
					},
				},
			},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `plugin binding "app-audit" references plugin "missing"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayMissingRateLimitPolicyRef(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
		},
		Routes: []resource.Route{
			{
				Metadata: resource.Metadata{Name: "app"},
				Spec: resource.RouteSpec{
					ParentRefs: []string{"public"},
				},
			},
		},
		PolicyBindings: []resource.PolicyBinding{
			{
				Metadata: resource.Metadata{Name: "app-quota"},
				Spec: resource.PolicyBindingSpec{
					TargetRef: resource.PolicyTargetRef{
						Kind: resource.KindRoute,
						Name: "app",
					},
					Policies: []resource.PolicyRef{
						{Kind: resource.KindRateLimitPolicy, Name: "missing"},
					},
				},
			},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `policy binding "app-quota" references rate limit policy "missing"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayMissingPolicyRef(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
		},
		Routes: []resource.Route{
			{
				Metadata: resource.Metadata{Name: "app"},
				Spec: resource.RouteSpec{
					ParentRefs: []string{"public"},
				},
			},
		},
		PolicyBindings: []resource.PolicyBinding{
			{
				Metadata: resource.Metadata{Name: "app-auth"},
				Spec: resource.PolicyBindingSpec{
					TargetRef: resource.PolicyTargetRef{
						Kind: resource.KindRoute,
						Name: "app",
					},
					Policies: []resource.PolicyRef{
						{Kind: resource.KindAuthPolicy, Name: "missing"},
					},
				},
			},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `policy binding "app-auth" references auth policy "missing"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayMissingPolicyBindingTarget(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
		},
		PolicyBindings: []resource.PolicyBinding{
			{
				Metadata: resource.Metadata{Name: "app-auth"},
				Spec: resource.PolicyBindingSpec{
					TargetRef: resource.PolicyTargetRef{
						Kind: resource.KindRoute,
						Name: "missing",
					},
				},
			},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `policy binding "app-auth" references route "missing"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayMissingGateway(t *testing.T) {
	_, err := (compiler.Compiler{}).CompileGateway(resource.Bundle{}, "missing")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `gateway "missing" not found`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayDuplicateGateway(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
			{Metadata: resource.Metadata{Name: "public"}},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `duplicate gateway "public"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayMissingRouteParent(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
		},
		Routes: []resource.Route{
			{
				Metadata: resource.Metadata{Name: "app"},
				Spec: resource.RouteSpec{
					ParentRefs: []string{"missing"},
				},
			},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `route "app" references gateway "missing"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayDuplicateRoute(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
		},
		Routes: []resource.Route{
			{Metadata: resource.Metadata{Name: "app"}},
			{Metadata: resource.Metadata{Name: "app"}},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `duplicate route "app"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayDuplicateUpstream(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
		},
		Upstreams: []resource.Upstream{
			{Metadata: resource.Metadata{Name: "app"}},
			{Metadata: resource.Metadata{Name: "app"}},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `duplicate upstream "app"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayMissingUpstream(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
		},
		Routes: []resource.Route{
			{
				Metadata: resource.Metadata{Name: "app"},
				Spec: resource.RouteSpec{
					ParentRefs: []string{"public"},
					Rules: []resource.RouteRule{
						{
							PathPrefix: "/app",
							UpstreamRefs: []resource.UpstreamRef{
								{Name: "missing", Weight: 100},
							},
						},
					},
				},
			},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `route "app" references upstream "missing"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}
