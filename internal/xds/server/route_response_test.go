package server

import (
	"encoding/json"
	"testing"

	udpatypev1 "github.com/cncf/xds/go/udpa/type/v1"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestResponseBuilderBuildRouteConfigsWithExactPath(t *testing.T) {
	configs := []snapshotConfig{
		{
			Gateway: "public",
			Version: "xds/public",
			Config: targetxds.Config{
				RouteConfigs: []targetxds.RouteConfig{
					{
						Name: "public/http/routes",
						VirtualHosts: []targetxds.VirtualHost{
							{
								Name:    "chat",
								Domains: []string{"api.example.com"},
								Routes: []targetxds.Route{
									{
										Name: "chat",
										Match: targetxds.RouteMatch{
											Path:       "/v1/chat/completions",
											PathPrefix: "/v1/chat",
										},
										WeightedClusters: []targetxds.WeightedCluster{
											{Name: "ai-provider/openai", Weight: 100},
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

	resources, err := (responseBuilder{}).buildRouteConfigs(configs)
	if err != nil {
		t.Fatalf("buildRouteConfigs() error = %v", err)
	}

	var routeConfig routev3.RouteConfiguration
	if err := resources[0].UnmarshalTo(&routeConfig); err != nil {
		t.Fatalf("UnmarshalTo(routeConfig) error = %v", err)
	}

	route := routeConfig.VirtualHosts[0].Routes[0]
	if route.GetMatch().GetPath() != "/v1/chat/completions" {
		t.Fatalf("Route path = %q, want exact AI path", route.GetMatch().GetPath())
	}
	clusterName := route.GetRoute().GetWeightedClusters().GetClusters()[0].GetName()
	if clusterName != "ai-provider/openai" {
		t.Fatalf("Route cluster = %q, want ai-provider/openai", clusterName)
	}
}

func TestResponseBuilderBuildRouteConfigsWithRoutePolicies(t *testing.T) {
	configs := []snapshotConfig{
		{
			Gateway: "public",
			Version: "xds/public",
			Config: targetxds.Config{
				RouteConfigs: []targetxds.RouteConfig{
					{
						Name: "public/http/routes",
						VirtualHosts: []targetxds.VirtualHost{
							{
								Name:    "app",
								Domains: []string{"example.com"},
								Routes: []targetxds.Route{
									{
										Name: "app",
										Match: targetxds.RouteMatch{
											PathPrefix: "/app",
										},
										TimeoutMillis: 1500,
										RequestHeadersToAdd: []targetxds.HeaderValue{
											{Name: "x-ingate-tenant", Value: "acme"},
										},
										RequestHeadersToRemove: []string{"x-debug-token"},
										RetryPolicy: &targetxds.RetryPolicy{
											Attempts:            2,
											PerTryTimeoutMillis: 500,
										},
										WeightedClusters: []targetxds.WeightedCluster{
											{Name: "app", Weight: 100},
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

	resources, err := (responseBuilder{}).buildRouteConfigs(configs)
	if err != nil {
		t.Fatalf("buildRouteConfigs() error = %v", err)
	}

	var routeConfig routev3.RouteConfiguration
	if err := resources[0].UnmarshalTo(&routeConfig); err != nil {
		t.Fatalf("UnmarshalTo(routeConfig) error = %v", err)
	}

	route := routeConfig.VirtualHosts[0].Routes[0]
	header := route.GetRequestHeadersToAdd()[0]
	if header.GetHeader().GetKey() != "x-ingate-tenant" || header.GetHeader().GetValue() != "acme" {
		t.Fatalf("RequestHeadersToAdd[0] = %v, want x-ingate-tenant=acme", header.GetHeader())
	}
	if header.GetAppendAction() != corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD {
		t.Fatalf("AppendAction = %v, want overwrite", header.GetAppendAction())
	}
	if route.GetRequestHeadersToRemove()[0] != "x-debug-token" {
		t.Fatalf("RequestHeadersToRemove[0] = %q, want x-debug-token", route.GetRequestHeadersToRemove()[0])
	}

	action := route.GetRoute()
	if action.GetTimeout().AsDuration().Milliseconds() != 1500 {
		t.Fatalf("Timeout = %s, want 1500ms", action.GetTimeout().AsDuration())
	}
	if action.GetRetryPolicy().GetNumRetries().GetValue() != 2 {
		t.Fatalf("NumRetries = %d, want 2", action.GetRetryPolicy().GetNumRetries().GetValue())
	}
	if action.GetRetryPolicy().GetPerTryTimeout().AsDuration().Milliseconds() != 500 {
		t.Fatalf("PerTryTimeout = %s, want 500ms", action.GetRetryPolicy().GetPerTryTimeout().AsDuration())
	}
}

func TestResponseBuilderBuildRouteConfigsWithManagedRateLimitPerRouteConfig(t *testing.T) {
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
										Match: targetxds.RouteMatch{
											PathPrefix: "/users",
											Methods:    []string{"GET"},
										},
										WeightedClusters: []targetxds.WeightedCluster{
											{Name: "users", Weight: 100},
										},
									},
								},
							},
						},
					},
				},
				ManagedRateLimit: &targetxds.ManagedRateLimit{
					Bindings: []targetxds.RateLimitBinding{
						{
							Name: "gateway-limit",
							Target: targetxds.RateLimitTarget{
								Kind: resource.KindGateway,
								Name: "public",
							},
							Policies: []targetxds.RateLimitPolicy{{Name: "policy-gateway"}},
						},
						{
							Name: "route-rule-limit",
							Target: targetxds.RateLimitTarget{
								Kind:     resource.KindRoute,
								Name:     "route-users",
								RuleName: "primary",
							},
							Policies: []targetxds.RateLimitPolicy{{Name: "policy-route-rule"}},
						},
						{
							Name: "other-rule-limit",
							Target: targetxds.RateLimitTarget{
								Kind:     resource.KindRoute,
								Name:     "route-users",
								RuleName: "secondary",
							},
							Policies: []targetxds.RateLimitPolicy{{Name: "policy-other-rule"}},
						},
					},
				},
			},
		},
	}

	resources, err := (responseBuilder{}).buildRouteConfigs(configs)
	if err != nil {
		t.Fatalf("buildRouteConfigs() error = %v", err)
	}

	var routeConfig routev3.RouteConfiguration
	if err := resources[0].UnmarshalTo(&routeConfig); err != nil {
		t.Fatalf("UnmarshalTo(routeConfig) error = %v", err)
	}

	route := routeConfig.VirtualHosts[0].Routes[0]
	if route.Name != "route-users/primary/GET" {
		t.Fatalf("Route.Name = %q, want route-users/primary/GET", route.Name)
	}
	configAny := route.GetTypedPerFilterConfig()[managedRateLimitHTTPFilterName]
	if configAny == nil {
		t.Fatalf("missing managed rate limit per-route config")
	}
	var typedConfig udpatypev1.TypedStruct
	if err := configAny.UnmarshalTo(&typedConfig); err != nil {
		t.Fatalf("UnmarshalTo(plugin config) error = %v", err)
	}
	if typedConfig.TypeUrl != managedRateLimitRouteTypeURL {
		t.Fatalf("TypeUrl = %q, want %q", typedConfig.TypeUrl, managedRateLimitRouteTypeURL)
	}
	rawConfig, err := protojson.Marshal(typedConfig.Value)
	if err != nil {
		t.Fatalf("Marshal(route config) error = %v", err)
	}
	var config managedRateLimitRouteConfig
	if err := json.Unmarshal(rawConfig, &config); err != nil {
		t.Fatalf("Unmarshal(managed route config) error = %v", err)
	}
	if config.SchemaVersion != managedRateLimitSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", config.SchemaVersion, managedRateLimitSchemaVersion)
	}
	if config.GatewayName != "public" || config.RouteName != "route-users" || config.RuleName != "primary" {
		t.Fatalf("route identity = %s/%s/%s, want public/route-users/primary", config.GatewayName, config.RouteName, config.RuleName)
	}
	if len(config.Bindings) != 2 {
		t.Fatalf("len(Bindings) = %d, want 2", len(config.Bindings))
	}
	if config.Bindings[0].Name != "gateway-limit" || config.Bindings[1].Name != "route-rule-limit" {
		t.Fatalf("Bindings = %+v, want gateway and route-rule bindings", config.Bindings)
	}
}

func TestResponseBuilderBuildRouteConfigsGroupsGatewaysByRuntimeEntry(t *testing.T) {
	configs := []snapshotConfig{
		{
			Gateway: "api",
			Version: "xds/api",
			Config: targetxds.Config{
				Listeners: []targetxds.Listener{
					{Name: "api/http", Protocol: "HTTP", Port: 8080, RouteConfigName: "api/http/routes"},
				},
				RouteConfigs: []targetxds.RouteConfig{
					{
						Name: "api/http/routes",
						VirtualHosts: []targetxds.VirtualHost{
							{
								Name:    "users",
								Domains: []string{"api.example.com"},
								Routes: []targetxds.Route{
									{
										Name: "users",
										WeightedClusters: []targetxds.WeightedCluster{
											{Name: "users", Weight: 100},
										},
									},
								},
							},
						},
					},
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
				RouteConfigs: []targetxds.RouteConfig{
					{
						Name: "ai/http/routes",
						VirtualHosts: []targetxds.VirtualHost{
							{
								Name:    "chat",
								Domains: []string{"ai.example.com"},
								Routes: []targetxds.Route{
									{
										Name: "chat",
										WeightedClusters: []targetxds.WeightedCluster{
											{Name: "chat", Weight: 100},
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

	resources, err := (responseBuilder{}).buildRouteConfigs(configs)
	if err != nil {
		t.Fatalf("buildRouteConfigs() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("len(resources) = %d, want 1 shared route config", len(resources))
	}

	var routeConfig routev3.RouteConfiguration
	if err := resources[0].UnmarshalTo(&routeConfig); err != nil {
		t.Fatalf("UnmarshalTo(routeConfig) error = %v", err)
	}
	if routeConfig.Name != "ingate/http-8080/routes" {
		t.Fatalf("RouteConfig name = %q, want shared runtime entry", routeConfig.Name)
	}
	if len(routeConfig.VirtualHosts) != 2 {
		t.Fatalf("len(VirtualHosts) = %d, want 2", len(routeConfig.VirtualHosts))
	}
	if routeConfig.VirtualHosts[0].Domains[0] != "api.example.com" {
		t.Fatalf("first domain = %q, want api.example.com", routeConfig.VirtualHosts[0].Domains[0])
	}
	if routeConfig.VirtualHosts[1].Domains[0] != "ai.example.com" {
		t.Fatalf("second domain = %q, want ai.example.com", routeConfig.VirtualHosts[1].Domains[0])
	}
}
