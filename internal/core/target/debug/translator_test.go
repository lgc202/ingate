package debug_test

import (
	"reflect"
	"testing"

	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/target/debug"
)

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
						PathPrefix: "/app",
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

	snapshot := (debug.Translator{}).Translate(logical)
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
						PathPrefix: "/app",
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
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config = %#v, want %#v", got, want)
	}
}
