package xds_test

import (
	"reflect"
	"testing"

	"github.com/lgc202/ingate/internal/core/ir"
	"github.com/lgc202/ingate/internal/core/target"
	"github.com/lgc202/ingate/internal/core/target/xds"
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
		Upstreams: []ir.LogicalUpstream{
			{
				Name: "app",
				Endpoints: []ir.LogicalEndpoint{
					{Address: "10.0.0.10", Port: 8080},
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
