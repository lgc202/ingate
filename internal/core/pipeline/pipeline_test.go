package pipeline_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lgc202/ingate-next/internal/core/pipeline"
	"github.com/lgc202/ingate-next/internal/core/resource"
	"github.com/lgc202/ingate-next/internal/core/target"
	"github.com/lgc202/ingate-next/internal/core/target/debug"
	"github.com/lgc202/ingate-next/internal/core/target/xds"
)

func TestPipelineBuildGatewaySnapshot(t *testing.T) {
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
							PathPrefix: "/app",
							UpstreamRefs: []resource.UpstreamRef{
								{Name: "app", Weight: 100},
							},
						},
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
	}

	snapshot, err := (pipeline.Pipeline{Translator: debug.Translator{}}).BuildGatewaySnapshot(bundle, "public")
	if err != nil {
		t.Fatalf("BuildGatewaySnapshot() error = %v", err)
	}
	if snapshot.Target != "debug" {
		t.Fatalf("Target = %q, want debug", snapshot.Target)
	}
	if snapshot.Gateway != "public" {
		t.Fatalf("Gateway = %q, want public", snapshot.Gateway)
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
		PolicyBindings: []debug.PolicyBinding{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config = %#v, want %#v", got, want)
	}
}

func TestPipelineBuildGatewaySnapshotCompileError(t *testing.T) {
	_, err := (pipeline.Pipeline{Translator: debug.Translator{}}).BuildGatewaySnapshot(resource.Bundle{}, "missing")
	if err == nil {
		t.Fatal("BuildGatewaySnapshot() error = nil")
	}
	if !strings.Contains(err.Error(), `compile gateway "missing": gateway "missing" not found`) {
		t.Fatalf("BuildGatewaySnapshot() error = %v", err)
	}
}

func TestPipelineBuildGatewaySnapshotForTarget(t *testing.T) {
	registry, err := target.NewRegistry(debug.Translator{}, xds.Translator{})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

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
					Rules: []resource.RouteRule{
						{
							PathPrefix: "/app",
							UpstreamRefs: []resource.UpstreamRef{
								{Name: "app", Weight: 100},
							},
						},
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
	}

	snapshot, err := (pipeline.Pipeline{Registry: registry}).BuildGatewaySnapshotForTarget(bundle, "public", "xds")
	if err != nil {
		t.Fatalf("BuildGatewaySnapshotForTarget() error = %v", err)
	}
	if snapshot.Target != "xds" {
		t.Fatalf("Target = %q, want xds", snapshot.Target)
	}
	if _, ok := snapshot.Config.(xds.Config); !ok {
		t.Fatalf("Config type = %T, want xds.Config", snapshot.Config)
	}
}

func TestPipelineBuildGatewaySnapshotForTargetMissingTarget(t *testing.T) {
	registry, err := target.NewRegistry(debug.Translator{})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	_, err = (pipeline.Pipeline{Registry: registry}).BuildGatewaySnapshotForTarget(resource.Bundle{}, "public", "xds")
	if err == nil {
		t.Fatal("BuildGatewaySnapshotForTarget() error = nil")
	}
	if !strings.Contains(err.Error(), `target "xds" not registered`) {
		t.Fatalf("BuildGatewaySnapshotForTarget() error = %v", err)
	}
}
