package server

import (
	"encoding/json"
	"testing"

	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	httpwasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	wasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestResponseBuilderBuildListenersWithWasmPlugin(t *testing.T) {
	pluginConfigRaw := json.RawMessage(`{"_rules_":[{"_match_route_":["chat"],"provider":{"type":"OpenAICompatible"}}]}`)
	configs := []snapshotConfig{
		{
			Gateway: "public",
			Version: "xds/public",
			Config: targetxds.Config{
				Listeners: []targetxds.Listener{
					{
						Name:            "public/http",
						Port:            80,
						RouteConfigName: "public/http/routes",
					},
				},
				Plugins: []targetxds.Plugin{
					{
						Name:    "ai-proxy",
						Runtime: resource.PluginRuntimeWasm,
						Image:   "/var/lib/ingate/plugins/ai-proxy.wasm",
					},
				},
				PluginBindings: []targetxds.PluginBinding{
					{
						Name: "chat-ai-proxy",
						Target: targetxds.PluginTarget{
							Kind: resource.KindGateway,
							Name: "public",
						},
						Priority:      100,
						FailurePolicy: resource.PluginFailurePolicyFailClose,
						Plugins: []targetxds.PluginRef{
							{
								Name:   "ai-proxy",
								Config: pluginConfigRaw,
							},
						},
					},
				},
			},
		},
	}

	resources, err := (responseBuilder{}).buildListeners(configs)
	if err != nil {
		t.Fatalf("buildListeners() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("len(resources) = %d, want 1", len(resources))
	}

	var listener listenerv3.Listener
	if err := resources[0].UnmarshalTo(&listener); err != nil {
		t.Fatalf("UnmarshalTo(listener) error = %v", err)
	}
	if len(listener.FilterChains) != 1 {
		t.Fatalf("len(FilterChains) = %d, want 1", len(listener.FilterChains))
	}
	filters := listener.FilterChains[0].Filters
	if len(filters) != 1 {
		t.Fatalf("len(Filters) = %d, want 1", len(filters))
	}

	var hcm hcmv3.HttpConnectionManager
	if err := filters[0].GetTypedConfig().UnmarshalTo(&hcm); err != nil {
		t.Fatalf("UnmarshalTo(hcm) error = %v", err)
	}
	if len(hcm.HttpFilters) != 2 {
		t.Fatalf("len(HttpFilters) = %d, want 2", len(hcm.HttpFilters))
	}
	if hcm.HttpFilters[0].Name != httpWasmFilterName {
		t.Fatalf("HttpFilters[0].Name = %q, want %q", hcm.HttpFilters[0].Name, httpWasmFilterName)
	}
	if hcm.HttpFilters[1].Name != httpRouterFilterName {
		t.Fatalf("HttpFilters[1].Name = %q, want %q", hcm.HttpFilters[1].Name, httpRouterFilterName)
	}

	var wasm httpwasmv3.Wasm
	if err := hcm.HttpFilters[0].GetTypedConfig().UnmarshalTo(&wasm); err != nil {
		t.Fatalf("UnmarshalTo(wasm) error = %v", err)
	}
	pluginConfig := wasm.GetConfig()
	if pluginConfig.GetName() != "ai-proxy" {
		t.Fatalf("PluginConfig.Name = %q, want ai-proxy", pluginConfig.GetName())
	}
	if pluginConfig.GetFailurePolicy() != wasmv3.FailurePolicy_FAIL_CLOSED {
		t.Fatalf("PluginConfig.FailurePolicy = %v, want fail close", pluginConfig.GetFailurePolicy())
	}
	if pluginConfig.GetVmConfig().GetCode().GetLocal().GetFilename() != "/var/lib/ingate/plugins/ai-proxy.wasm" {
		t.Fatalf("PluginConfig code filename = %q", pluginConfig.GetVmConfig().GetCode().GetLocal().GetFilename())
	}

	var pluginJSON wrapperspb.StringValue
	if err := pluginConfig.GetConfiguration().UnmarshalTo(&pluginJSON); err != nil {
		t.Fatalf("UnmarshalTo(plugin config) error = %v", err)
	}
	if pluginJSON.Value != string(pluginConfigRaw) {
		t.Fatalf("Plugin configuration = %q, want provider config", pluginJSON.Value)
	}
}

func TestResponseBuilderBuildListenersWithManagedRateLimit(t *testing.T) {
	configs := []snapshotConfig{
		{
			Gateway: "public",
			Version: "xds/public",
			Config: targetxds.Config{
				Listeners: []targetxds.Listener{
					{Name: "public/http", Protocol: "HTTP", Port: 80, RouteConfigName: "public/http/routes"},
				},
				ManagedRateLimit: &targetxds.ManagedRateLimit{
					Bindings: []targetxds.RateLimitBinding{
						{Name: "public-limit"},
					},
					RedisStores: []targetxds.RateLimitRedisStore{
						{Name: "redis-main", DisplayName: "主 Redis", Address: "redis.example.com:6379"},
					},
				},
				Plugins: []targetxds.Plugin{
					{
						Name:    "audit",
						Runtime: resource.PluginRuntimeWasm,
						Image:   "/var/lib/ingate/plugins/audit.wasm",
					},
				},
				PluginBindings: []targetxds.PluginBinding{
					{
						Name:          "audit",
						Priority:      100,
						FailurePolicy: resource.PluginFailurePolicyFailOpen,
						Plugins: []targetxds.PluginRef{
							{Name: "audit", Config: json.RawMessage(`{"enabled":true}`)},
						},
					},
				},
			},
		},
	}

	resources, err := (responseBuilder{}).buildListeners(configs)
	if err != nil {
		t.Fatalf("buildListeners() error = %v", err)
	}

	var listener listenerv3.Listener
	if err := resources[0].UnmarshalTo(&listener); err != nil {
		t.Fatalf("UnmarshalTo(listener) error = %v", err)
	}
	var hcm hcmv3.HttpConnectionManager
	if err := listener.FilterChains[0].Filters[0].GetTypedConfig().UnmarshalTo(&hcm); err != nil {
		t.Fatalf("UnmarshalTo(hcm) error = %v", err)
	}
	if len(hcm.HttpFilters) != 3 {
		t.Fatalf("len(HttpFilters) = %d, want 3", len(hcm.HttpFilters))
	}
	if hcm.HttpFilters[0].Name != managedRateLimitHTTPFilterName {
		t.Fatalf("HttpFilters[0].Name = %q, want managed rate limit", hcm.HttpFilters[0].Name)
	}
	if hcm.HttpFilters[1].Name != httpWasmFilterName {
		t.Fatalf("HttpFilters[1].Name = %q, want user wasm", hcm.HttpFilters[1].Name)
	}
	if hcm.HttpFilters[2].Name != httpRouterFilterName {
		t.Fatalf("HttpFilters[2].Name = %q, want router", hcm.HttpFilters[2].Name)
	}

	var wasm httpwasmv3.Wasm
	if err := hcm.HttpFilters[0].GetTypedConfig().UnmarshalTo(&wasm); err != nil {
		t.Fatalf("UnmarshalTo(wasm) error = %v", err)
	}
	pluginConfig := wasm.GetConfig()
	if pluginConfig.GetName() != managedRateLimitPluginName {
		t.Fatalf("PluginConfig.Name = %q, want %q", pluginConfig.GetName(), managedRateLimitPluginName)
	}
	if pluginConfig.GetFailurePolicy() != wasmv3.FailurePolicy_FAIL_CLOSED {
		t.Fatalf("PluginConfig.FailurePolicy = %v, want fail close", pluginConfig.GetFailurePolicy())
	}
	if pluginConfig.GetVmConfig().GetCode().GetLocal().GetFilename() != managedRateLimitPluginPath {
		t.Fatalf("PluginConfig filename = %q, want %q", pluginConfig.GetVmConfig().GetCode().GetLocal().GetFilename(), managedRateLimitPluginPath)
	}

	var pluginJSON wrapperspb.StringValue
	if err := pluginConfig.GetConfiguration().UnmarshalTo(&pluginJSON); err != nil {
		t.Fatalf("UnmarshalTo(plugin config) error = %v", err)
	}
	var config managedRateLimitFilterConfig
	if err := json.Unmarshal([]byte(pluginJSON.Value), &config); err != nil {
		t.Fatalf("Unmarshal(managed config) error = %v", err)
	}
	if config.SchemaVersion != managedRateLimitSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", config.SchemaVersion, managedRateLimitSchemaVersion)
	}
	if len(config.RedisStores) != 1 || config.RedisStores[0].Name != "redis-main" {
		t.Fatalf("RedisStores = %+v, want redis-main", config.RedisStores)
	}
}

func TestResponseBuilderBuildListenersGroupsGatewaysByRuntimeEntry(t *testing.T) {
	configs := []snapshotConfig{
		{
			Gateway: "api",
			Version: "xds/api",
			Config: targetxds.Config{
				Listeners: []targetxds.Listener{
					{Name: "api/http", Protocol: "HTTP", Port: 8080, RouteConfigName: "api/http/routes"},
				},
			},
		},
		{
			Gateway: "ai",
			Version: "xds/ai",
			Config: targetxds.Config{
				Listeners: []targetxds.Listener{
					{Name: "ai/http", Protocol: "HTTP", Port: 8080, RouteConfigName: "ai/http/routes"},
				},
			},
		},
	}

	resources, err := (responseBuilder{}).buildListeners(configs)
	if err != nil {
		t.Fatalf("buildListeners() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("len(resources) = %d, want 1 shared listener", len(resources))
	}

	var listener listenerv3.Listener
	if err := resources[0].UnmarshalTo(&listener); err != nil {
		t.Fatalf("UnmarshalTo(listener) error = %v", err)
	}
	if listener.Name != "ingate/http-8080" {
		t.Fatalf("Listener name = %q, want shared runtime entry", listener.Name)
	}
	if listener.GetAddress().GetSocketAddress().GetPortValue() != 8080 {
		t.Fatalf("Listener port = %d, want 8080", listener.GetAddress().GetSocketAddress().GetPortValue())
	}

	var hcm hcmv3.HttpConnectionManager
	filter := listener.FilterChains[0].Filters[0]
	if err := filter.GetTypedConfig().UnmarshalTo(&hcm); err != nil {
		t.Fatalf("UnmarshalTo(hcm) error = %v", err)
	}
	if hcm.GetRds().GetRouteConfigName() != "ingate/http-8080/routes" {
		t.Fatalf("RouteConfigName = %q, want shared route config", hcm.GetRds().GetRouteConfigName())
	}
}
