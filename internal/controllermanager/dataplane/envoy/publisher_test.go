package envoy

import (
	"context"
	"testing"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

func TestPublishReplacesCompleteConfiguration(t *testing.T) {
	publisher := NewPublisher("", "")
	config := dataplane.Config{
		Listeners: []dataplane.Listener{{
			Port: 8080,
			Hosts: []dataplane.Host{{
				Name:     "gateway/main/listener/http",
				Hostname: "example.com",
				Routes: []dataplane.Route{{
					Name:        "api",
					PathType:    dataplane.PathMatchPrefix,
					Path:        "/api",
					BackendName: "backend",
				}},
			}},
		}},
		Backends: []dataplane.Backend{{
			Name:      "backend",
			Endpoints: []dataplane.Endpoint{{Address: "192.0.2.1", Port: 9000, Weight: 1}},
		}},
	}

	if err := publisher.Publish(context.Background(), config); err != nil {
		t.Fatalf("Publish(config) error = %v, want nil", err)
	}

	first, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(after config) error = %v, want nil", err)
	}

	for _, typeURL := range []resourcev3.Type{
		resourcev3.ListenerType, resourcev3.RouteType, resourcev3.ClusterType, resourcev3.EndpointType,
	} {
		if got, want := len(first.GetResources(typeURL)), 1; got != want {
			t.Errorf("Publish(config) %s resources = %d, want %d", typeURL, got, want)
		}
	}

	if err := publisher.Publish(context.Background(), config); err != nil {
		t.Fatalf("Publish(config) repeated error = %v, want nil", err)
	}

	again, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(after repeated publish) error = %v, want nil", err)
	}

	if again != first {
		t.Error("Publish(config) repeated replaced an unchanged snapshot")
	}

	config.Listeners[0].Hosts[0].Routes[0].PathType = "Unknown"
	if err := publisher.Publish(context.Background(), config); err == nil {
		t.Error("Publish(invalid route) error = nil, want build error")
	}

	afterInvalid, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(after invalid publish) error = %v, want nil", err)
	}

	if afterInvalid != first {
		t.Error("Publish(invalid route) replaced the last valid snapshot")
	}

	config.Listeners[0].Hosts[0].Routes[0].PathType = dataplane.PathMatchPrefix
	config.Backends[0].Endpoints[0].Address = "192.0.2.2"
	if err := publisher.Publish(context.Background(), config); err != nil {
		t.Fatalf("Publish(changed endpoint) error = %v, want nil", err)
	}

	changed, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(after endpoint change) error = %v, want nil", err)
	}

	if got, want := changed.GetVersion(resourcev3.EndpointType), first.GetVersion(resourcev3.EndpointType); got == want {
		t.Errorf("Publish(changed endpoint) EDS version = %q, want different from %q", got, want)
	}

	for _, typeURL := range []resourcev3.Type{resourcev3.ListenerType, resourcev3.RouteType, resourcev3.ClusterType} {
		if got, want := changed.GetVersion(typeURL), first.GetVersion(typeURL); got != want {
			t.Errorf("Publish(changed endpoint) %s version = %q, want %q", typeURL, got, want)
		}
	}

	if err := publisher.Publish(context.Background(), dataplane.Config{}); err != nil {
		t.Fatalf("Publish(empty) error = %v, want nil", err)
	}

	empty, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(after empty publish) error = %v, want nil", err)
	}

	for _, typeURL := range []resourcev3.Type{
		resourcev3.ListenerType, resourcev3.RouteType, resourcev3.ClusterType, resourcev3.EndpointType,
	} {
		if got := len(empty.GetResources(typeURL)); got != 0 {
			t.Errorf("Publish(empty) %s resources = %d, want 0", typeURL, got)
		}
	}
}

func TestPublishDirectResponseAndRecovery(t *testing.T) {
	publisher := NewPublisher("", "")
	config := dataplane.Config{
		Listeners: []dataplane.Listener{{
			Port: 8080,
			Hosts: []dataplane.Host{{
				Name: "edge/http",
				Routes: []dataplane.Route{{
					Name:     "missing",
					PathType: dataplane.PathMatchPrefix,
					Path:     "/api",
				}},
			}},
		}},
	}
	if err := publisher.Publish(t.Context(), config); err != nil {
		t.Fatalf("Publish(direct response): %v", err)
	}

	snapshot, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatal(err)
	}

	routes := snapshot.GetResources(resourcev3.RouteType)["route/8080"].(*routev3.RouteConfiguration)
	if got := routes.VirtualHosts[0].Routes[0].GetDirectResponse().GetStatus(); got != 500 {
		t.Errorf("direct response status = %d, want 500", got)
	}

	config.Listeners[0].Hosts[0].Routes[0].BackendName = "backend"
	config.Backends = []dataplane.Backend{{
		Name:      "backend",
		Endpoints: []dataplane.Endpoint{{Address: "192.0.2.1", Port: 9000, Weight: 1}},
	}}
	if err := publisher.Publish(t.Context(), config); err != nil {
		t.Fatalf("Publish(recovered backend): %v", err)
	}

	snapshot, err = publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatal(err)
	}

	routes = snapshot.GetResources(resourcev3.RouteType)["route/8080"].(*routev3.RouteConfiguration)
	if got := routes.VirtualHosts[0].Routes[0].GetRoute().GetCluster(); got != "backend/backend" {
		t.Errorf("recovered route cluster = %q, want backend/backend", got)
	}
}

func TestNewSnapshotRejectsMissingCluster(t *testing.T) {
	_, err := newSnapshot(resourceSet{routes: []*routev3.RouteConfiguration{{
		Name: "route/8080",
		VirtualHosts: []*routev3.VirtualHost{{
			Name: "host", Domains: []string{"example.com"},
			Routes: []*routev3.Route{{
				Name: "api",
				Action: &routev3.Route_Route{Route: &routev3.RouteAction{
					ClusterSpecifier: &routev3.RouteAction_Cluster{Cluster: "backend/missing"},
				}},
			}},
		}},
	}}})
	if err == nil {
		t.Error("newSnapshot(route without cluster) error = nil, want reference error")
	}
}
