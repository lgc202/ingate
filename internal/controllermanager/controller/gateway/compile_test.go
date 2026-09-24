package gateway

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestCompileGroupsHostsByPort(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{
		gatewayResource("second", 2, listener("http", 8080, "b.example.com")),
		gatewayResource("first", 1,
			listener("admin", 9090, ""),
			listener("http", 8080, "a.example.com"),
		),
	}
	routes := []gatewayv1alpha1.Route{
		routeResource("missing", 1, "first", "http", "unknown", gatewayv1alpha1.PathMatchPrefix),
		routeResource("valid", 2, "second", "http", "backend", gatewayv1alpha1.PathMatchExact),
	}
	upstreams := []gatewayv1alpha1.Upstream{upstreamResource("backend")}

	got := compile(resolve(gateways, routes, upstreams))
	want := dataplane.Config{
		Listeners: []dataplane.Listener{
			{
				Port: 8080,
				Hosts: []dataplane.Host{
					{Name: "gateway/first/listener/http", Hostname: "a.example.com"},
					{
						Name:     "gateway/second/listener/http",
						Hostname: "b.example.com",
						Routes: []dataplane.Route{{
							Name: "valid", PathType: dataplane.PathMatchExact,
							Path: "/api", UpstreamName: "backend",
						}},
					},
				},
			},
			{
				Port:  9090,
				Hosts: []dataplane.Host{{Name: "gateway/first/listener/admin"}},
			},
		},
		Upstreams: []dataplane.Upstream{{
			Name: "backend",
			Endpoints: []dataplane.Endpoint{{
				Address: "127.0.0.1", Port: 8081, Weight: 1,
			}},
		}},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("compile(resolve(two Gateways, one missing Upstream)) mismatch (-want +got):\n%s", diff)
	}
}

func TestCompileEmpty(t *testing.T) {
	got := compile(resolve(nil, nil, nil))
	if diff := cmp.Diff(dataplane.Config{}, got); diff != "" {
		t.Errorf("compile(resolve(empty resources)) mismatch (-want +got):\n%s", diff)
	}
}

func TestCompileNormalizesPathPrefix(t *testing.T) {
	route := routeResource("api", 1, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	route.Spec.Path.Value = "/api/"

	config := compile(resolve(
		[]gatewayv1alpha1.Gateway{gatewayResource("edge", 1, listener("http", 8080, ""))},
		[]gatewayv1alpha1.Route{route},
		[]gatewayv1alpha1.Upstream{upstreamResource("backend")},
	))
	want := []dataplane.Listener{{
		Port: 8080,
		Hosts: []dataplane.Host{{
			Name: "gateway/edge/listener/http",
			Routes: []dataplane.Route{{
				Name: "api", PathType: dataplane.PathMatchPrefix,
				Path: "/api", UpstreamName: "backend",
			}},
		}},
	}}

	if diff := cmp.Diff(want, config.Listeners); diff != "" {
		t.Errorf("compile(PathPrefix /api/) listeners mismatch (-want +got):\n%s", diff)
	}
}
