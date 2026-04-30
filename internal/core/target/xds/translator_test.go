package xds_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/lgc202/ingate/internal/core/ir"
	"github.com/lgc202/ingate/internal/core/target"
	"github.com/lgc202/ingate/internal/core/target/xds"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestTranslatorImplementsTargetTranslator(t *testing.T) {
	var translator target.Translator = xds.Translator{}
	if translator.Target() != "xds" {
		t.Fatalf("Target() = %q, want xds", translator.Target())
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
				Hostnames:  []string{"api.example.com"},
				Path:       "/v1/chat/completions",
				PathPrefix: "/v1/chat",
				Model:      "gpt-4.1-mini",
				Models: []ir.LogicalAIModelRef{
					{Name: "chat-fast", Weight: 100},
				},
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
				Name:    "ai-proxy",
				Runtime: resource.PluginRuntimeWasm,
				Version: "v1",
				Image:   "oci://example.com/ai-proxy:v1",
			},
		},
		PluginBindings: []ir.LogicalPluginBinding{
			{
				Name: "chat-ai-proxy",
				Target: ir.LogicalPluginTarget{
					Kind: resource.KindAIRoute,
					Name: "chat",
				},
				Phase:         resource.PluginPhaseBeforeProviderCall,
				Priority:      100,
				FailurePolicy: resource.PluginFailurePolicyFailClose,
				Plugins: []ir.LogicalPluginRef{
					{
						Name: "ai-proxy",
					},
				},
			},
		},
	}

	snapshot, err := (xds.Translator{}).Translate(logical)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if snapshot.Target != "xds" {
		t.Fatalf("Target = %q, want xds", snapshot.Target)
	}
	if snapshot.Gateway != "public" {
		t.Fatalf("Gateway = %q, want public", snapshot.Gateway)
	}
	if snapshot.Version == "" {
		t.Fatal("Version is empty")
	}

	got, ok := snapshot.Config.(xds.Config)
	if !ok {
		t.Fatalf("Config type = %T, want xds.Config", snapshot.Config)
	}
	want := xds.Config{
		Listeners: []xds.Listener{
			{
				Name:            "public/http",
				Protocol:        "HTTP",
				Port:            80,
				Hostname:        "example.com",
				RouteConfigName: "public/http/routes",
			},
		},
		RouteConfigs: []xds.RouteConfig{
			{
				Name: "public/http/routes",
				VirtualHosts: []xds.VirtualHost{
					{
						Name:    "app",
						Domains: []string{"example.com"},
						Routes: []xds.Route{
							{
								Name: "app",
								Match: xds.RouteMatch{
									PathPrefix: "/app",
									Methods:    []string{"GET", "POST"},
									Headers: []xds.HeaderMatch{
										{Name: "x-tenant", Value: "acme"},
									},
								},
								TimeoutMillis: 3000,
								WeightedClusters: []xds.WeightedCluster{
									{Name: "app", Weight: 100},
								},
							},
						},
					},
				},
			},
		},
		Clusters: []xds.Cluster{
			{
				Name: "app",
			},
		},
		EndpointAssignments: []xds.EndpointAssignment{
			{
				ClusterName: "app",
				Endpoints: []xds.Endpoint{
					{Address: "10.0.0.10", Port: 8080},
				},
			},
		},
		AIRoutes: []xds.AIRoute{
			{
				Name:    "chat",
				Domains: []string{"api.example.com"},
				Match: xds.AIRouteMatch{
					Path:       "/v1/chat/completions",
					PathPrefix: "/v1/chat",
				},
				Model: "gpt-4.1-mini",
				Models: []xds.AIModelRef{
					{Name: "chat-fast", Weight: 100},
				},
				Providers:  []xds.AIProviderRef{},
				PolicyRefs: []string{"ai-default"},
			},
		},
		AIProviders: []xds.AIProvider{
			{
				Name:     "openai",
				Type:     resource.AIProviderTypeOpenAICompatible,
				Endpoint: "https://api.openai.com/v1",
				Models:   []string{"gpt-4.1-mini"},
			},
		},
		AIModels: []xds.AIModel{
			{
				Name:          "chat-fast",
				ProviderRef:   "openai",
				ProviderModel: "gpt-4.1-mini",
				Capabilities:  []string{"chat", "stream"},
			},
		},
		AIPolicies: []xds.AIPolicy{
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
		Plugins: []xds.Plugin{
			{
				Name:    "ai-proxy",
				Runtime: resource.PluginRuntimeWasm,
				Version: "v1",
				Image:   "oci://example.com/ai-proxy:v1",
			},
		},
		PluginBindings: []xds.PluginBinding{
			{
				Name: "chat-ai-proxy",
				Target: xds.PluginTarget{
					Kind: resource.KindGateway,
					Name: "public",
				},
				Phase:         resource.PluginPhaseBeforeProviderCall,
				Priority:      100,
				FailurePolicy: resource.PluginFailurePolicyFailClose,
				Plugins: []xds.PluginRef{
					{
						Name:   "ai-proxy",
						Config: json.RawMessage(`{"_rules_":[{"_match_domain_":["api.example.com"],"_match_route_":["chat"],"model":"gpt-4.1-mini","models":[{"name":"chat-fast","weight":100}],"policyRefs":["ai-default"],"provider":{"endpoint":"https://api.openai.com/v1","modelMapping":{"*":"gpt-4.1-mini"},"name":"openai","type":"OpenAICompatible"}}]}`),
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config = %#v, want %#v", got, want)
	}
}

func TestTranslatorTranslateRouteWithoutHostnameUsesWildcardDomain(t *testing.T) {
	logical := ir.LogicalGateway{
		Name: "public",
		Listeners: []ir.LogicalListener{
			{Name: "http", Protocol: "HTTP", Port: 80},
		},
		Routes: []ir.LogicalRoute{
			{
				Name: "app",
				Rules: []ir.LogicalRouteRule{
					{
						PathPrefix: "/app",
						Upstreams: []ir.LogicalUpstreamRef{
							{Name: "app", Weight: 100},
						},
					},
				},
			},
		},
		Upstreams: []ir.LogicalUpstream{
			{Name: "app"},
		},
	}

	snapshot, err := (xds.Translator{}).Translate(logical)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	got := snapshot.Config.(xds.Config)
	want := []string{"*"}

	if !reflect.DeepEqual(got.RouteConfigs[0].VirtualHosts[0].Domains, want) {
		t.Fatalf("Domains = %#v, want %#v", got.RouteConfigs[0].VirtualHosts[0].Domains, want)
	}
}
