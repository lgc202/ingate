package envoy

import (
	"testing"

	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

func TestBuildResourcesSeparatesNamespacedTargets(t *testing.T) {
	targets := []struct {
		namespace string
		address   string
	}{
		{namespace: "team-a", address: "192.0.2.1"},
		{namespace: "team-b", address: "192.0.2.2"},
	}
	config := dataplane.Config{Listeners: []dataplane.Listener{{Port: 8080}}}
	for _, target := range targets {
		config.Listeners[0].Hosts = append(config.Listeners[0].Hosts, dataplane.Host{
			Name:     "gateway/" + target.namespace + "/edge/listener/http",
			Hostname: target.namespace + ".example.com",
			Routes: []dataplane.Route{{
				Name:        target.namespace + "/api/rule/0/match/0",
				PathType:    dataplane.PathMatchPrefix,
				Path:        "/api",
				BackendName: target.namespace + "/backend",
			}},
		})
		config.Backends = append(config.Backends, dataplane.Backend{
			Name:      target.namespace + "/backend",
			Endpoints: []dataplane.Endpoint{{Address: target.address, Port: 8081, Weight: 1}},
		})
	}

	resources, err := buildResources(config)
	if err != nil {
		t.Fatalf("buildResources(namespaced targets) error = %v", err)
	}
	snapshot, err := newSnapshot(resources)
	if err != nil {
		t.Fatalf("newSnapshot(namespaced targets) error = %v", err)
	}
	for _, typeURL := range []resourcev3.Type{resourcev3.ClusterType, resourcev3.EndpointType} {
		if got := len(snapshot.GetResources(typeURL)); got != len(targets) {
			t.Errorf("snapshot %s resources = %d, want %d", typeURL, got, len(targets))
		}
	}
	if len(resources.listeners) != 1 || len(resources.routes) != 1 {
		t.Fatalf("resources listeners/routes = %d/%d, want one shared listener and route configuration", len(resources.listeners), len(resources.routes))
	}
	hosts := resources.routes[0].VirtualHosts
	if len(hosts) != len(targets) {
		t.Fatalf("virtual hosts = %d, want %d", len(hosts), len(targets))
	}
	for i, target := range targets {
		clusterName := "backend/" + target.namespace + "/backend"
		if snapshot.GetResources(resourcev3.ClusterType)[clusterName] == nil {
			t.Errorf("snapshot missing cluster %q", clusterName)
		}
		assignment, ok := snapshot.GetResources(resourcev3.EndpointType)[clusterName].(*endpointv3.ClusterLoadAssignment)
		if !ok {
			t.Fatalf("snapshot missing endpoint assignment %q", clusterName)
		}
		if len(assignment.Endpoints) != 1 || len(assignment.Endpoints[0].LbEndpoints) != 1 {
			t.Fatalf("assignment %q did not contain exactly one endpoint", clusterName)
		}
		endpoint := assignment.Endpoints[0].LbEndpoints[0].GetEndpoint()
		if got := endpoint.GetAddress().GetSocketAddress().GetAddress(); got != target.address {
			t.Errorf("assignment %q address = %q, want %q", clusterName, got, target.address)
		}
		if got := hosts[i].Name; got != config.Listeners[0].Hosts[i].Name {
			t.Errorf("virtual host[%d] name = %q, want %q", i, got, config.Listeners[0].Hosts[i].Name)
		}
		if len(hosts[i].Routes) != 1 {
			t.Fatalf("virtual host[%d] routes = %d, want 1", i, len(hosts[i].Routes))
		}
		if got := hosts[i].Routes[0].GetRoute().GetCluster(); got != clusterName {
			t.Errorf("virtual host[%d] target = %q, want %q", i, got, clusterName)
		}
	}
}
