package envoy

import (
	"slices"
	"strings"
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/google/go-cmp/cmp"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

func TestNewClusterEndpointAssignment(t *testing.T) {
	for _, tt := range []struct {
		name      string
		endpoints []dataplane.Endpoint
		want      []dataplane.Endpoint
		wantType  clusterv3.Cluster_DiscoveryType
	}{
		{
			name: "IP",
			endpoints: []dataplane.Endpoint{
				{Address: "2001:db8::1", Port: 9000, Weight: 1},
				{Address: "192.0.2.1", Port: 9001, Weight: 3},
				{Address: "192.0.2.1", Port: 9000, Weight: 2},
				{Address: "192.0.2.1", Port: 9000, Weight: 1},
			},
			want: []dataplane.Endpoint{
				{Address: "192.0.2.1", Port: 9000, Weight: 1},
				{Address: "192.0.2.1", Port: 9000, Weight: 2},
				{Address: "192.0.2.1", Port: 9001, Weight: 3},
				{Address: "2001:db8::1", Port: 9000, Weight: 1},
			},
			wantType: clusterv3.Cluster_EDS,
		},
		{
			name: "mixed IP and DNS",
			endpoints: []dataplane.Endpoint{
				{Address: "backend.example.com", Port: 9000, Weight: 2},
				{Address: "192.0.2.1", Port: 9001, Weight: 1},
			},
			want: []dataplane.Endpoint{
				{Address: "192.0.2.1", Port: 9001, Weight: 1},
				{Address: "backend.example.com", Port: 9000, Weight: 2},
			},
			wantType: clusterv3.Cluster_STRICT_DNS,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := slices.Clone(tt.endpoints)
			cluster, assignment, err := newCluster(dataplane.Backend{Name: "backend", Endpoints: tt.endpoints})
			if err != nil {
				t.Fatalf("newCluster(%v) error = %v, want nil", tt.endpoints, err)
			}

			if got := cluster.GetType(); got != tt.wantType {
				t.Errorf("newCluster(%v) discovery type = %v, want %v", tt.endpoints, got, tt.wantType)
			}

			if got := assignment.ClusterName; got != "backend/backend" {
				t.Errorf("newCluster(%v) assignment name = %q, want backend/backend", tt.endpoints, got)
			}

			if len(assignment.Endpoints) != 1 {
				t.Fatalf("newCluster(%v) localities = %d, want 1", tt.endpoints, len(assignment.Endpoints))
			}

			var got []dataplane.Endpoint
			for _, endpoint := range assignment.Endpoints[0].LbEndpoints {
				address := endpoint.GetEndpoint().GetAddress().GetSocketAddress()
				got = append(got, dataplane.Endpoint{
					Address: address.GetAddress(), Port: int32(address.GetPortValue()),
					Weight: int32(endpoint.GetLoadBalancingWeight().GetValue()),
				})
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("newCluster(%v) endpoints mismatch (-want +got):\n%s", tt.endpoints, diff)
			}

			if diff := cmp.Diff(before, tt.endpoints); diff != "" {
				t.Errorf("newCluster(%v) modified input (-want +got):\n%s", before, diff)
			}

			if tt.wantType == clusterv3.Cluster_STRICT_DNS {
				if cluster.LoadAssignment != assignment || cluster.EdsClusterConfig != nil {
					t.Errorf("newCluster(%v) did not embed DNS assignment or unexpectedly configured EDS", tt.endpoints)
				}
			} else if cluster.LoadAssignment != nil || cluster.GetEdsClusterConfig().GetServiceName() != "backend/backend" {
				t.Errorf("newCluster(%v) did not configure EDS service or unexpectedly embedded assignment", tt.endpoints)
			}
		})
	}
}

func TestNewClusterRejectsInvalidEndpoint(t *testing.T) {
	for _, tt := range []struct {
		name       string
		endpoint   dataplane.Endpoint
		wantPrefix string
	}{
		{name: "IP weight", endpoint: dataplane.Endpoint{Address: "192.0.2.1", Port: 9000}, wantPrefix: "validate endpoints for \"backend/backend\": "},
		{name: "DNS weight", endpoint: dataplane.Endpoint{Address: "backend.example.com", Port: 9000}, wantPrefix: "validate cluster \"backend/backend\": "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cluster, assignment, err := newCluster(dataplane.Backend{Name: "backend", Endpoints: []dataplane.Endpoint{tt.endpoint}})
			if err == nil {
				t.Fatalf("newCluster(%+v) error = nil, want validation error", tt.endpoint)
			}

			if cluster != nil || assignment != nil {
				t.Errorf("newCluster(%+v) = (%v, %v), want nil results on error", tt.endpoint, cluster, assignment)
			}

			if !strings.HasPrefix(err.Error(), tt.wantPrefix) {
				t.Errorf("newCluster(%+v) error = %v, want prefix %q", tt.endpoint, err, tt.wantPrefix)
			}
		})
	}
}

func TestInvalidBackendKeepsMatchBeforeFallback(t *testing.T) {
	listener := dataplane.Listener{Port: 8080, Hosts: []dataplane.Host{{Name: "edge", Routes: []dataplane.Route{
		{Name: "fallback", PathType: dataplane.PathMatchPrefix, Path: "/", BackendName: "valid"},
		{Name: "z-first-rule", PathType: dataplane.PathMatchPrefix, Path: "/api"},
		{Name: "a-second-rule", PathType: dataplane.PathMatchPrefix, Path: "/api", BackendName: "valid"},
	}}}}
	config, err := newRouteConfiguration(listener)
	if err != nil {
		t.Fatalf("newRouteConfiguration: %v", err)
	}

	routes := config.VirtualHosts[0].Routes
	if len(routes) != 3 {
		t.Fatalf("routes = %d, want invalid match and both valid candidates", len(routes))
	}

	if routes[0].Name != "z-first-rule" || routes[0].GetDirectResponse().GetStatus() != 500 || routes[0].GetRoute() != nil {
		t.Errorf("first rule = %v, want 500 without forwarding", routes[0])
	}

	if routes[0].Match.GetPathSeparatedPrefix() != "/api" {
		t.Errorf("invalid match lost segment boundary: %v", routes[0].Match)
	}

	if routes[1].Name != "a-second-rule" || routes[1].GetRoute().GetCluster() != "backend/valid" || routes[2].Name != "fallback" {
		t.Errorf("candidate ordering = %v", routes)
	}

	listener.Hosts[0].Routes[1].BackendName = "restored"
	restored, err := newRouteConfiguration(listener)
	if err != nil {
		t.Fatal(err)
	}

	if first := restored.VirtualHosts[0].Routes[0]; first.GetDirectResponse() != nil || first.GetRoute().GetCluster() != "backend/restored" {
		t.Errorf("restored target = %v", first)
	}

	if listener.Hosts[0].Routes[0].Name != "fallback" {
		t.Error("adapter mutated input order")
	}
}

func TestBuildResources(t *testing.T) {
	config := dataplane.Config{
		Listeners: []dataplane.Listener{{
			Port: 8080,
			Hosts: []dataplane.Host{{
				Name: "gateway/main/listener/http", Hostname: "example.com",
				Routes: []dataplane.Route{
					{Name: "root", PathType: dataplane.PathMatchPrefix, Path: "/", BackendName: "ip"},
					{Name: "prefix", PathType: dataplane.PathMatchPrefix, Path: "/api", BackendName: "dns"},
					{Name: "exact", PathType: dataplane.PathMatchExact, Path: "/api", BackendName: "ip"},
					{Name: "docs", PathType: dataplane.PathMatchPrefix, Path: "/docs", BackendName: "ip"},
				},
			}},
		}},
		Backends: []dataplane.Backend{
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

	if got := routes[0].GetRoute().GetCluster(); got != "backend/ip" {
		t.Errorf("buildResources(config) exact route cluster = %q, want backend/ip", got)
	}

	if got := snapshot.GetResources(resourcev3.RouteType)["route/8080"].(*routev3.RouteConfiguration).VirtualHosts[0].Domains[0]; got != "example.com" {
		t.Errorf("newSnapshot(resources) virtual host domain = %q, want example.com", got)
	}
}

func TestNewSnapshotRejectsMissingEDSAssignment(t *testing.T) {
	cluster, _, err := newCluster(dataplane.Backend{
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
