package pipeline_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lgc202/ingate/internal/core/pipeline"
	"github.com/lgc202/ingate/internal/core/target"
	"github.com/lgc202/ingate/internal/core/target/debug"
	"github.com/lgc202/ingate/internal/core/target/xds"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testGateway(name string, listeners ...resource.Listener) resource.Gateway {
	return resource.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: resource.GatewaySpec{
			Enabled:   true,
			Listeners: listeners,
		},
	}
}

func TestPipelineBuildGatewaySnapshot(t *testing.T) {
	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "public"},
				Spec: resource.GatewaySpec{
					Enabled: true,
					Listeners: []resource.Listener{
						{Name: "http", Protocol: resource.ProtocolHTTP, Port: 80},
					},
					HostBindings: []resource.HostBinding{
						{Hostname: "example.com", ListenerRefs: []string{"http"}},
					},
				},
			},
		},
		Routes: []resource.Route{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.RouteSpec{
					Enabled:    true,
					ParentRefs: []resource.ParentRef{{Name: "public"}},
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
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.UpstreamSpec{
					Endpoints: []resource.Endpoint{
						{Address: "10.0.0.10", Port: 8080, Enabled: true},
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
		AuthPolicies:      []debug.AuthPolicy{},
		RateLimitPolicies: []debug.RateLimitPolicy{},
		PolicyBindings:    []debug.PolicyBinding{},
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
				ObjectMeta: metav1.ObjectMeta{Name: "public"},
				Spec: resource.GatewaySpec{
					Enabled: true,
					Listeners: []resource.Listener{
						{Name: "http", Protocol: resource.ProtocolHTTP, Port: 80},
					},
					HostBindings: []resource.HostBinding{
						{Hostname: "example.com", ListenerRefs: []string{"http"}},
					},
				},
			},
		},
		Routes: []resource.Route{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.RouteSpec{
					Enabled:    true,
					ParentRefs: []resource.ParentRef{{Name: "public"}},
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
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.UpstreamSpec{
					Endpoints: []resource.Endpoint{
						{Address: "10.0.0.10", Port: 8080, Enabled: true},
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

func TestPipelineBuildGatewaySnapshotsForTarget(t *testing.T) {
	registry, err := target.NewRegistry(debug.Translator{})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	bundle := resource.Bundle{
		Gateways: []resource.Gateway{
			testGateway("public", resource.Listener{Name: "http", Protocol: resource.ProtocolHTTP, Port: 80}),
			testGateway("admin", resource.Listener{Name: "http", Protocol: resource.ProtocolHTTP, Port: 8080}),
		},
	}

	snapshots, err := (pipeline.Pipeline{Registry: registry}).BuildGatewaySnapshotsForTarget(bundle, "debug")
	if err != nil {
		t.Fatalf("BuildGatewaySnapshotsForTarget() error = %v", err)
	}

	wantGateways := []string{"public", "admin"}
	if len(snapshots) != len(wantGateways) {
		t.Fatalf("BuildGatewaySnapshotsForTarget() len = %d, want %d", len(snapshots), len(wantGateways))
	}
	for i, wantGateway := range wantGateways {
		if snapshots[i].Gateway != wantGateway {
			t.Fatalf("BuildGatewaySnapshotsForTarget()[%d].Gateway = %q, want %q", i, snapshots[i].Gateway, wantGateway)
		}
		if snapshots[i].Target != "debug" {
			t.Fatalf("BuildGatewaySnapshotsForTarget()[%d].Target = %q, want debug", i, snapshots[i].Target)
		}
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
