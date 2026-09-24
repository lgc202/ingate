package envoy

import (
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	cachetypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

func TestBuildResources(t *testing.T) {
	config := dataplane.Config{
		Listeners: []dataplane.Listener{{
			Port: 8080,
			Hosts: []dataplane.Host{{
				Name: "gateway/main/listener/http", Hostname: "example.com",
				Routes: []dataplane.Route{
					{Name: "root", PathType: dataplane.PathMatchPrefix, Path: "/", UpstreamName: "ip"},
					{Name: "prefix", PathType: dataplane.PathMatchPrefix, Path: "/api", UpstreamName: "dns"},
					{Name: "exact", PathType: dataplane.PathMatchExact, Path: "/api", UpstreamName: "ip"},
					{Name: "docs", PathType: dataplane.PathMatchPrefix, Path: "/docs", UpstreamName: "ip"},
				},
			}},
		}},
		Upstreams: []dataplane.Upstream{
			{Name: "ip", Endpoints: []dataplane.Endpoint{{Address: "192.0.2.1", Port: 9000, Weight: 3}}},
			{Name: "dns", Endpoints: []dataplane.Endpoint{{Address: "backend.example.com", Port: 9001, Weight: 2}}},
		},
	}

	resources, err := buildResources(config)
	if err != nil {
		t.Fatalf("buildResources(config) error = %v, want nil", err)
	}
	if got, want := len(resources.listeners), 1; got != want {
		t.Fatalf("buildResources(config) listeners = %d, want %d", got, want)
	}
	if got, want := len(resources.routes), 1; got != want {
		t.Fatalf("buildResources(config) route configurations = %d, want %d", got, want)
	}
	if got, want := len(resources.clusters), 2; got != want {
		t.Fatalf("buildResources(config) clusters = %d, want %d", got, want)
	}
	if got, want := len(resources.endpoints), 1; got != want {
		t.Fatalf("buildResources(config) EDS assignments = %d, want %d", got, want)
	}
	if len(resources.routes) != 1 || len(resources.routes[0].VirtualHosts) != 1 {
		t.Fatal("buildResources(config) did not create the expected virtual host")
	}
	routes := resources.routes[0].VirtualHosts[0].Routes
	if got, want := len(routes), 4; got != want {
		t.Fatalf("buildResources(config) virtual host routes = %d, want %d", got, want)
	}
	for i, want := range []string{"exact", "docs", "prefix", "root"} {
		if got := routes[i].Name; got != want {
			t.Errorf("buildResources(config) route[%d] = %q, want %q", i, got, want)
		}
	}
	if got := routes[2].Match.GetPathSeparatedPrefix(); got != "/api" {
		t.Errorf("buildResources(config) prefix match = %q, want /api", got)
	}
	if got := routes[0].Match.GetPath(); got != "/api" {
		t.Errorf("buildResources(config) exact match = %q, want /api", got)
	}
	if got := routes[3].Match.GetPrefix(); got != "/" {
		t.Errorf("buildResources(config) root match = %q, want /", got)
	}
	if got := routes[1].Match.GetPathSeparatedPrefix(); got != "/docs" {
		t.Errorf("buildResources(config) path-separated prefix = %q, want /docs", got)
	}
	if got := resources.clusters[0].GetType(); got != clusterv3.Cluster_EDS {
		t.Errorf("buildResources(config) IP cluster type = %v, want EDS", got)
	}
	if got := resources.clusters[1].GetType(); got != clusterv3.Cluster_STRICT_DNS {
		t.Errorf("buildResources(config) DNS cluster type = %v, want STRICT_DNS", got)
	}
	if got := resources.endpoints[0].Endpoints[0].LbEndpoints[0].GetLoadBalancingWeight().GetValue(); got != 3 {
		t.Errorf("buildResources(config) endpoint weight = %d, want 3", got)
	}

	snapshot, err := newSnapshot(resources)
	if err != nil {
		t.Fatalf("newSnapshot(resources) error = %v, want nil", err)
	}
	if got, want := len(snapshot.GetResources(resourcev3.EndpointType)), 1; got != want {
		t.Errorf("newSnapshot(resources) endpoints = %d, want %d", got, want)
	}
	if got, want := len(snapshot.GetResources(resourcev3.ClusterType)), 2; got != want {
		t.Errorf("newSnapshot(resources) clusters = %d, want %d", got, want)
	}
	if got := routes[0].GetRoute().GetCluster(); got != "upstream/ip" {
		t.Errorf("buildResources(config) exact route cluster = %q, want upstream/ip", got)
	}
	if got := snapshot.GetResources(resourcev3.RouteType)["route/8080"].(*routev3.RouteConfiguration).VirtualHosts[0].Domains[0]; got != "example.com" {
		t.Errorf("newSnapshot(resources) virtual host domain = %q, want example.com", got)
	}
}

func TestNewSnapshotRejectsMissingEDSAssignment(t *testing.T) {
	cluster, _, err := newCluster(dataplane.Upstream{
		Name: "backend", Endpoints: []dataplane.Endpoint{{Address: "192.0.2.1", Port: 9000, Weight: 1}},
	})
	if err != nil {
		t.Fatalf("newCluster(backend) error = %v, want nil", err)
	}
	_, err = newSnapshot(resourceSet{clusters: []*clusterv3.Cluster{cluster}})
	if err == nil {
		t.Error("newSnapshot(cluster without EDS assignment) error = nil, want reference error")
	}
}

func TestNewRouteMatchRejectsUnknownType(t *testing.T) {
	_, err := newRouteMatch(dataplane.Route{PathType: "Unknown", Path: "/"})
	if err == nil {
		t.Error("newRouteMatch(Unknown) error = nil, want unsupported type error")
	}
}

func TestCompareRoutesOrdersUnknownTypesConsistently(t *testing.T) {
	a := dataplane.Route{PathType: "OtherA"}
	b := dataplane.Route{PathType: "OtherB"}
	forward := compareRoutes(a, b)
	reverse := compareRoutes(b, a)
	if forward >= 0 || reverse <= 0 {
		t.Errorf("compareRoutes(OtherA, OtherB) = %d, reverse = %d; want negative and positive", forward, reverse)
	}
}

func TestResourceVersionDependsOnContent(t *testing.T) {
	first := []cachetypes.Resource{&routev3.RouteConfiguration{Name: "route/8080"}}
	version, err := resourceVersion(first)
	if err != nil {
		t.Fatalf("resourceVersion(first) error = %v, want nil", err)
	}
	again, err := resourceVersion(first)
	if err != nil {
		t.Fatalf("resourceVersion(first) repeated error = %v, want nil", err)
	}
	if version != again {
		t.Errorf("resourceVersion(first) repeated = %q, want %q", again, version)
	}
	changed, err := resourceVersion([]cachetypes.Resource{&routev3.RouteConfiguration{Name: "route/9090"}})
	if err != nil {
		t.Fatalf("resourceVersion(changed) error = %v, want nil", err)
	}
	if version == changed {
		t.Errorf("resourceVersion(changed) = %q, want different from %q", changed, version)
	}
}
