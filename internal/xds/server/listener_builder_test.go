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
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestResponseBuilderBuildListenersWithRateLimit(t *testing.T) {
	configs := []snapshotConfig{
		{
			Gateway: "public",
			Version: "xds/public",
			Config: targetxds.Config{
				Listeners: []targetxds.Listener{
					{Name: "public/http", Protocol: "HTTP", Port: 80, RouteConfigName: "public/http/routes"},
				},
				RouteConfigs: []targetxds.RouteConfig{
					{
						Name: "public/http/routes",
						VirtualHosts: []targetxds.VirtualHost{
							{
								Name:    "app",
								Domains: []string{"example.com"},
								Routes: []targetxds.Route{
									{
										GatewayName: "public",
										Name:        "route-users",
										RuleName:    "primary",
										WeightedClusters: []targetxds.WeightedCluster{
											{Name: "users", Weight: 100},
										},
									},
								},
							},
						},
					},
				},
				RateLimit: &targetxds.RateLimitConfig{
					Bindings: []pluginratelimit.Binding{
						{
							Name: "gateway-limit",
							Target: pluginratelimit.Target{
								Kind: string(resource.KindGateway),
								Name: "public",
							},
						},
						{
							Name: "route-rule-limit",
							Target: pluginratelimit.Target{
								Kind:     string(resource.KindRoute),
								Name:     "route-users",
								RuleName: "primary",
							},
						},
					},
					RedisStores: []pluginratelimit.RedisStore{
						{Name: "redis-main", DisplayName: "主 Redis", Address: "redis.example.com:6379"},
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
	if len(hcm.HttpFilters) != 2 {
		t.Fatalf("len(HttpFilters) = %d, want 2", len(hcm.HttpFilters))
	}
	if hcm.HttpFilters[0].Name != rateLimitHTTPFilterName {
		t.Fatalf("HttpFilters[0].Name = %q, want rate limit", hcm.HttpFilters[0].Name)
	}
	if hcm.HttpFilters[1].Name != httpRouterFilterName {
		t.Fatalf("HttpFilters[1].Name = %q, want router", hcm.HttpFilters[1].Name)
	}

	var wasm httpwasmv3.Wasm
	if err := hcm.HttpFilters[0].GetTypedConfig().UnmarshalTo(&wasm); err != nil {
		t.Fatalf("UnmarshalTo(wasm) error = %v", err)
	}
	pluginConfig := wasm.GetConfig()
	if pluginConfig.GetName() != rateLimitPluginName {
		t.Fatalf("PluginConfig.Name = %q, want %q", pluginConfig.GetName(), rateLimitPluginName)
	}
	if pluginConfig.GetFailurePolicy() != wasmv3.FailurePolicy_FAIL_CLOSED {
		t.Fatalf("PluginConfig.FailurePolicy = %v, want fail close", pluginConfig.GetFailurePolicy())
	}
	if pluginConfig.GetVmConfig().GetCode().GetLocal().GetFilename() != rateLimitPluginPath {
		t.Fatalf("PluginConfig filename = %q, want %q", pluginConfig.GetVmConfig().GetCode().GetLocal().GetFilename(), rateLimitPluginPath)
	}

	var pluginJSON wrapperspb.StringValue
	if err := pluginConfig.GetConfiguration().UnmarshalTo(&pluginJSON); err != nil {
		t.Fatalf("UnmarshalTo(plugin config) error = %v", err)
	}
	var config pluginratelimit.PluginConfig
	if err := json.Unmarshal([]byte(pluginJSON.Value), &config); err != nil {
		t.Fatalf("Unmarshal(rate limit config) error = %v", err)
	}
	if config.SchemaVersion != rateLimitSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", config.SchemaVersion, rateLimitSchemaVersion)
	}
	if len(config.RedisStores) != 1 || config.RedisStores[0].Name != "redis-main" {
		t.Fatalf("RedisStores = %+v, want redis-main", config.RedisStores)
	}
	if len(config.Routes) != 1 {
		t.Fatalf("len(Routes) = %d, want 1", len(config.Routes))
	}
	routeConfig := config.Routes[0]
	if routeConfig.GatewayName != "public" || routeConfig.RouteName != "route-users" || routeConfig.RuleName != "primary" {
		t.Fatalf("route identity = %s/%s/%s, want public/route-users/primary", routeConfig.GatewayName, routeConfig.RouteName, routeConfig.RuleName)
	}
	if len(routeConfig.Bindings) != 2 {
		t.Fatalf("len(Bindings) = %d, want 2", len(routeConfig.Bindings))
	}
	if routeConfig.Bindings[0].Name != "gateway-limit" || routeConfig.Bindings[1].Name != "route-rule-limit" {
		t.Fatalf("Bindings = %+v, want gateway and route-rule bindings", routeConfig.Bindings)
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
