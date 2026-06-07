package compiler_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/lgc202/ingate/internal/core/compiler"
	"github.com/lgc202/ingate/internal/core/ir"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func testGateway(name string) resource.Gateway {
	return resource.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: resource.GatewaySpec{
			Enabled: true,
		},
	}
}

func TestCompilerCompileGateway(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "public"},
				Spec: resource.GatewaySpec{
					Enabled: true,
					Listeners: []resource.Listener{
						{Name: "http", Protocol: resource.ProtocolHTTP, Port: 80},
					},
					HostBindings: []resource.HostBinding{
						{Hostname: "example.com", ListenerRefs: []string{"http"}},
					},
				},
			},
		},
		Routes: []resource.Route{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.RouteSpec{
					Enabled:    true,
					ParentRefs: []resource.ParentRef{{Name: "public"}},
					Hostnames:  []string{"example.com"},
					Rules: []resource.RouteRule{
						{
							PathPrefix: "/app",
							Methods:    []string{"GET", "POST"},
							Timeout: &resource.RouteTimeout{
								RequestMillis: 1500,
							},
							Retry: &resource.RouteRetry{
								Attempts:            2,
								PerTryTimeoutMillis: 500,
							},
							Headers: []resource.HeaderMatch{
								{Name: "x-tenant", Value: "acme"},
							},
							Filters: []resource.RouteFilter{
								{
									Type: resource.RouteFilterRequestHeaderModifier,
									RequestHeaderModifier: &resource.HeaderModifier{
										Set: []resource.HeaderValue{
											{Name: "x-ingate-tenant", Value: "acme"},
										},
										Remove: []string{"x-debug-token"},
									},
								},
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
				ObjectMeta: metav1.ObjectMeta{Name: "chat"},
				Spec: resource.AIRouteSpec{
					ParentRefs: []string{"public"},
					Hostnames:  []string{"api.example.com"},
					Path:       "/v1/chat/completions",
					PathPrefix: "/v1/chat/completions",
					Model:      "gpt-4.1-mini",
					Models: []resource.AIModelRef{
						{Name: "chat-fast", Weight: 100},
					},
					PolicyRefs: []string{"ai-default"},
				},
			},
		},
		Upstreams: []resource.Upstream{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.UpstreamSpec{
					Endpoints: []resource.Endpoint{
						{Address: "10.0.0.10", Port: 8080, Enabled: true},
					},
				},
			},
		},
		AIProviders: []resource.AIProvider{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "openai"},
				Spec: resource.AIProviderSpec{
					Type:     resource.AIProviderTypeOpenAICompatible,
					Endpoint: "https://api.openai.com/v1",
					Models:   []string{"gpt-4.1-mini"},
				},
			},
		},
		AIModels: []resource.AIModel{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "chat-fast"},
				Spec: resource.AIModelSpec{
					ProviderRef:   "openai",
					ProviderModel: "gpt-4.1-mini",
					Capabilities:  []string{"chat", "stream"},
				},
			},
		},
		AIPolicies: []resource.AIPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "ai-default"},
				Spec: resource.AIPolicySpec{
					ExecutionTarget: resource.AIExecutionTargetTypeWasm,
					TimeoutMillis:   30000,
					Retry: resource.AIRetryPolicy{
						Attempts: 2,
					},
					Fallback: resource.AIFallbackPolicy{
						Enabled: true,
						Models:  []string{"chat-backup"},
					},
					Usage: resource.AIUsagePolicy{
						Enabled: true,
					},
				},
			},
		},
		Plugins: []resource.Plugin{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "audit-log"},
				Spec: resource.PluginSpec{
					Runtime:  resource.PluginRuntimeExternal,
					Version:  "v1",
					Endpoint: "dns:///audit-plugin:9000",
				},
			},
		},
		AuthPolicies: []resource.AuthPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "required"},
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
				ObjectMeta: metav1.ObjectMeta{Name: "app-quota"},
				Spec: resource.RateLimitPolicySpec{
					Enabled: true,
					Mode:    resource.RateLimitModeLocal,
					Rules: []resource.RateLimitRule{
						{
							Name: "consumer-minute",
							Key: resource.RateLimitKey{
								Parts: []resource.RateLimitKeyPart{
									{Type: resource.RateLimitKeyTypeHeader, Name: "x-consumer-id"},
								},
							},
							Limit: resource.RateLimitQuota{
								Requests:      100,
								WindowSeconds: 60,
							},
							Algorithm: resource.RateLimitAlgorithmFixedWindow,
						},
					},
				},
			},
		},
		PluginBindings: []resource.PluginBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-audit"},
				Spec: resource.PluginBindingSpec{
					TargetRef: resource.PluginTargetRef{
						Kind: resource.KindAIRoute,
						Name: "chat",
					},
					Phase:         resource.PluginPhaseBeforeProviderCall,
					Priority:      100,
					FailurePolicy: resource.PluginFailurePolicyFailClose,
					Plugins: []resource.PluginRef{
						{
							Name:   "audit-log",
							Config: runtime.RawExtension{Raw: []byte(`{"mode":"audit"}`)},
						},
					},
				},
			},
		},
		PolicyBindings: []resource.PolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-auth"},
				Spec: resource.PolicyBindingSpec{
					Enabled: true,
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
						TimeoutMillis: 1500,
						Headers: []ir.LogicalHeaderMatch{
							{Name: "x-tenant", Value: "acme"},
						},
						RequestHeadersToAdd: []ir.LogicalHeaderValue{
							{Name: "x-ingate-tenant", Value: "acme"},
						},
						RequestHeadersToRemove: []string{"x-debug-token"},
						Retry: ir.LogicalRetryPolicy{
							Attempts:            2,
							PerTryTimeoutMillis: 500,
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
				Hostnames:  []string{"api.example.com"},
				Path:       "/v1/chat/completions",
				PathPrefix: "/v1/chat/completions",
				Model:      "gpt-4.1-mini",
				Models: []ir.LogicalAIModelRef{
					{Name: "chat-fast", Weight: 100},
				},
				Providers:  []ir.LogicalAIProviderRef{},
				PolicyRefs: []string{"ai-default"},
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
		AIModels: []ir.LogicalAIModel{
			{
				Name:          "chat-fast",
				ProviderRef:   "openai",
				ProviderModel: "gpt-4.1-mini",
				Capabilities:  []string{"chat", "stream"},
			},
		},
		AIPolicies: []ir.LogicalAIPolicy{
			{
				Name:            "ai-default",
				ExecutionTarget: resource.AIExecutionTargetTypeWasm,
				TimeoutMillis:   30000,
				RetryAttempts:   2,
				FallbackEnabled: true,
				FallbackModels:  []string{"chat-backup"},
				UsageEnabled:    true,
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
		PluginBindings: []ir.LogicalPluginBinding{
			{
				Name: "app-audit",
				Target: ir.LogicalPluginTarget{
					Kind: resource.KindAIRoute,
					Name: "chat",
				},
				Phase:         resource.PluginPhaseBeforeProviderCall,
				Priority:      100,
				FailurePolicy: resource.PluginFailurePolicyFailClose,
				Plugins: []ir.LogicalPluginRef{
					{
						Name:   "audit-log",
						Config: json.RawMessage(`{"mode":"audit"}`),
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
			testGateway("public"),
		},
		AIRoutes: []resource.AIRoute{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "chat"},
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

func TestCompilerCompileGatewayUnsupportedRouteFilter(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			testGateway("public"),
		},
		Routes: []resource.Route{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.RouteSpec{
					Enabled:    true,
					ParentRefs: []resource.ParentRef{{Name: "public"}},
					Rules: []resource.RouteRule{
						{
							PathPrefix: "/app",
							Filters: []resource.RouteFilter{
								{Type: resource.RouteFilterURLRewrite},
							},
							UpstreamRefs: []resource.UpstreamRef{
								{Name: "app", Weight: 100},
							},
						},
					},
				},
			},
		},
		Upstreams: []resource.Upstream{
			{ObjectMeta: metav1.ObjectMeta{Name: "app"}},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `route "app" has unsupported route filter "URLRewrite"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayAIRouteWithoutProvider(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			testGateway("public"),
		},
		AIRoutes: []resource.AIRoute{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "chat"},
				Spec: resource.AIRouteSpec{
					ParentRefs: []string{"public"},
					PathPrefix: "/v1/chat/completions",
					Model:      "gpt-4.1-mini",
				},
			},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `ai route "chat" has no ai models or ai providers`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayAIRouteInvalidProviderWeight(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			testGateway("public"),
		},
		AIProviders: []resource.AIProvider{
			{ObjectMeta: metav1.ObjectMeta{Name: "openai"}},
		},
		AIRoutes: []resource.AIRoute{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "chat"},
				Spec: resource.AIRouteSpec{
					ParentRefs: []string{"public"},
					PathPrefix: "/v1/chat/completions",
					Model:      "gpt-4.1-mini",
					ProviderRefs: []resource.AIProviderRef{
						{Name: "openai"},
					},
				},
			},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `ai route "chat" provider "openai" has invalid weight 0`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayMissingPluginRef(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			testGateway("public"),
		},
		Routes: []resource.Route{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.RouteSpec{
					Enabled:    true,
					ParentRefs: []resource.ParentRef{{Name: "public"}},
				},
			},
		},
		PluginBindings: []resource.PluginBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-audit"},
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
			testGateway("public"),
		},
		Routes: []resource.Route{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.RouteSpec{
					Enabled:    true,
					ParentRefs: []resource.ParentRef{{Name: "public"}},
				},
			},
		},
		PolicyBindings: []resource.PolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-quota"},
				Spec: resource.PolicyBindingSpec{
					Enabled: true,
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
			testGateway("public"),
		},
		Routes: []resource.Route{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.RouteSpec{
					Enabled:    true,
					ParentRefs: []resource.ParentRef{{Name: "public"}},
				},
			},
		},
		PolicyBindings: []resource.PolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-auth"},
				Spec: resource.PolicyBindingSpec{
					Enabled: true,
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
			testGateway("public"),
		},
		PolicyBindings: []resource.PolicyBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-auth"},
				Spec: resource.PolicyBindingSpec{
					Enabled: true,
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
			testGateway("public"),
			testGateway("public"),
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

func TestCompilerCompileGatewayDuplicateRoute(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			testGateway("public"),
		},
		Routes: []resource.Route{
			{ObjectMeta: metav1.ObjectMeta{Name: "app"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "app"}},
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
			testGateway("public"),
		},
		Upstreams: []resource.Upstream{
			{ObjectMeta: metav1.ObjectMeta{Name: "app"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "app"}},
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
			testGateway("public"),
		},
		Routes: []resource.Route{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.RouteSpec{
					Enabled:    true,
					ParentRefs: []resource.ParentRef{{Name: "public"}},
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
