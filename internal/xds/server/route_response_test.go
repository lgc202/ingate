package server

import (
	"testing"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
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
