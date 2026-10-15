package envoy

import (
	"slices"
	"testing"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/proto"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestAIIngressSharesHTTPListener(t *testing.T) {
	for _, order := range []string{"HTTP first", "AI first"} {
		t.Run(order, func(t *testing.T) {
			publisher := NewPublisher("/run/ingate/ingress.sock", "/run/ingate/egress.sock")
			ai := aiFixture(t)
			hostName := ai.Listeners[0].Hosts[0].Name
			httpConfig := dataplane.Config{Listeners: []dataplane.Listener{{Port: 8080, Hosts: []dataplane.Host{
				{Name: hostName, Hostname: "gateway.example.com", Routes: []dataplane.Route{
					{Name: "ordinary exact", PathType: dataplane.PathMatchExact, Path: "/v1/chat/completions"},
					{Name: "ordinary prefix", PathType: dataplane.PathMatchPrefix, Path: "/"},
				}},
				{Name: "gateway/team-b/other/listener/api", Hostname: "other.example.com"},
			}}}}
			httpConfig.Listeners = append(httpConfig.Listeners, dataplane.Listener{
				Port: 9090, Hosts: []dataplane.Host{{Name: hostName, Hostname: "gateway.example.com"}},
			})
			publishHTTP := func() error { return publisher.Publish(t.Context(), httpConfig) }
			publishAI := func() error { return publisher.PublishAI(t.Context(), ai) }
			steps := []func() error{publishHTTP, publishAI}
			if order == "AI first" {
				slices.Reverse(steps)
			}

			for _, publish := range steps {
				if err := publish(); err != nil {
					t.Fatalf("Publish(%s) error = %v", order, err)
				}
			}
			snapshot, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
			if err != nil {
				t.Fatalf("GetSnapshot(%s) error = %v", order, err)
			}

			if len(snapshot.GetResources(resourcev3.ListenerType)) != 3 || len(snapshot.GetResources(resourcev3.RouteType)) != 3 {
				t.Fatalf("Publish(%s) listeners/routes = %d/%d, want two shared public listeners and private egress", order, len(snapshot.GetResources(resourcev3.ListenerType)), len(snapshot.GetResources(resourcev3.RouteType)))
			}
			otherPort := snapshot.GetResources(resourcev3.RouteType)[routeName(9090)].(*routev3.RouteConfiguration)
			if len(otherPort.VirtualHosts[0].Routes) != 0 {
				t.Errorf("Publish(%s) added model routes to unbound port: %v", order, otherPort)
			}
			routes := snapshot.GetResources(resourcev3.RouteType)[routeName(8080)].(*routev3.RouteConfiguration)
			if err := routes.ValidateAll(); err != nil {
				t.Errorf("ValidateAll(merged ingress) error = %v", err)
			}
			host := routes.VirtualHosts[0]
			if len(host.Routes) != 3 || len(routes.VirtualHosts[1].Routes) != 0 {
				t.Fatalf("Publish(%s) host route counts = %d/%d, want 3/0 without cross-host entry", order, len(host.Routes), len(routes.VirtualHosts[1].Routes))
			}
			entry := host.Routes[0]
			if entry.Match.GetPath() != "/v1/chat/completions" || len(entry.Match.Headers) != 1 || entry.Match.Headers[0].Name != ":method" || entry.Match.Headers[0].GetStringMatch().GetExact() != "POST" {
				t.Errorf("model entry match = %v, want only exact Chat Completions POST", entry.Match)
			}
			if host.Routes[1].Name != "ordinary exact" || host.Routes[2].Name != "ordinary prefix" {
				t.Errorf("model entry changed ordinary route order: %v", host.Routes)
			}
			if entry.GetRoute().GetCluster() != aiIngressName || entry.GetRoute().Timeout == nil || entry.GetRoute().Timeout.AsDuration() != 0 || entry.GetRoute().RetryPolicy != nil {
				t.Errorf("model entry action = %v, want fixed Go cluster without total timeout or retries", entry.GetRoute())
			}
			wantHeaders := map[string]string{aidataplane.ListenerPortHeader: "8080", aidataplane.HostNameHeader: hostName}
			if len(entry.RequestHeadersToAdd) != len(wantHeaders) {
				t.Fatalf("model entry headers = %v, want exactly port and host", entry.RequestHeadersToAdd)
			}
			for _, header := range entry.RequestHeadersToAdd {
				want, exists := wantHeaders[header.Header.Key]
				if !exists || header.Header.Value != want || header.AppendAction != corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD {
					t.Errorf("model entry header = %v, want literal overwrite from %v", header, wantHeaders)
				}
			}
			if !slices.Equal(entry.RequestHeadersToRemove, []string{aidataplane.TargetHeader}) {
				t.Errorf("model entry removed headers = %v, want client target removed", entry.RequestHeadersToRemove)
			}
			cluster := snapshot.GetResources(resourcev3.ClusterType)[aiIngressName].(*clusterv3.Cluster)
			pipe := cluster.LoadAssignment.Endpoints[0].LbEndpoints[0].GetEndpoint().Address.GetPipe()
			if cluster.GetType() != clusterv3.Cluster_STATIC || pipe.GetPath() != "/run/ingate/ingress.sock" {
				t.Errorf("model entry cluster = %v, want fixed Unix upstream", cluster)
			}
			before := proto.Clone(routes)

			if err := publishHTTP(); err != nil {
				t.Fatalf("Publish(repeated HTTP) error = %v", err)
			}
			repeated, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
			if err != nil {
				t.Fatalf("GetSnapshot(repeated HTTP) error = %v", err)
			}
			currentRoutes := repeated.GetResources(resourcev3.RouteType)[routeName(8080)].(*routev3.RouteConfiguration)
			if !proto.Equal(before, currentRoutes) {
				t.Errorf("Publish(repeated HTTP) duplicated or changed model routes: %v", currentRoutes)
			}

			ai.Listeners[0].Hosts[0].ModelRoutes = []aidataplane.ModelRoute{}
			if err := publishAI(); err != nil {
				t.Fatalf("PublishAI(MCP only) error = %v", err)
			}
			revoked, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
			if err != nil {
				t.Fatalf("GetSnapshot(revoked model) error = %v", err)
			}

			ordinary := revoked.GetResources(resourcev3.RouteType)[routeName(8080)].(*routev3.RouteConfiguration)
			if len(ordinary.VirtualHosts[0].Routes) != 2 || ordinary.VirtualHosts[0].Routes[0].Name != "ordinary exact" || len(revoked.GetResources(resourcev3.ClusterType)) != 0 {
				t.Errorf("PublishAI(MCP only) retained model ingress or lost HTTP routes: %v", ordinary)
			}
			if !proto.Equal(before, routes) {
				t.Error("Publish mutated the previously installed public routes")
			}
		})
	}
}

func TestAIIngressRequiresDedicatedAbsoluteSocket(t *testing.T) {
	for _, socket := range []string{"", "relative.sock", "/run/invalid\x00.sock", "/run/ingate/./egress.sock"} {
		t.Run(socket, func(t *testing.T) {
			config := aiFixture(t)

			_, err := buildAIResources(config, socket, "/run/ingate/egress.sock")
			if err == nil {
				t.Errorf("buildAIResources(ingress %q) error = nil, want invalid or shared path rejected", socket)
			}
		})
	}
}
