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
