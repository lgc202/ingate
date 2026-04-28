package compiler_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lgc202/ingate-next/internal/core/compiler"
	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/resource"
)

func TestCompilerCompileGateway(t *testing.T) {
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

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CompileGateway() = %#v, want %#v", got, want)
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

func TestCompilerCompileGatewayMissingRouteParent(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
		},
		Routes: []resource.Route{
			{
				Metadata: resource.Metadata{Name: "app"},
				Spec: resource.RouteSpec{
					ParentRefs: []string{"missing"},
				},
			},
		},
	}

	_, err := (compiler.Compiler{}).CompileGateway(bundle, "public")
	if err == nil {
		t.Fatal("CompileGateway() error = nil")
	}
	if !strings.Contains(err.Error(), `route "app" references gateway "missing"`) {
		t.Fatalf("CompileGateway() error = %v", err)
	}
}

func TestCompilerCompileGatewayMissingUpstream(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{Metadata: resource.Metadata{Name: "public"}},
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
