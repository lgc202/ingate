package envoy

import (
	"slices"
	"testing"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestPublishReusesNamedResources(t *testing.T) {
	config := twoPortConfig()
	publisher := NewPublisher("", "")
	first := publishTestSnapshot(t, publisher, config)
	initialRoute := proto.Clone(first.GetResources(resourcev3.RouteType)["route/8080"])
	for _, typeURL := range []resourcev3.Type{
		resourcev3.ListenerType, resourcev3.RouteType, resourcev3.ClusterType, resourcev3.EndpointType,
	} {
		for name, resource := range first.GetResources(typeURL) {
			encoded, err := cachev3.MarshalResource(resource)
			if err != nil {
				t.Fatalf("MarshalResource(%s/%s) error = %v", typeURL, name, err)
			}

			if got, want := first.GetVersionMap(typeURL)[name], cachev3.HashResource(encoded); got != want {
				t.Errorf("initial resource version(%s/%s) = %q, want native hash %q", typeURL, name, got, want)
			}
		}
	}

	config.Listeners[0].Hosts[0].Routes[0].Path = "/v2/orders"
	changedRoute := publishTestSnapshot(t, publisher, config)

	assertNamedResourceChanges(t, first, changedRoute, resourcev3.RouteType, "route/8080")
	if diff := cmp.Diff(initialRoute, first.GetResources(resourcev3.RouteType)["route/8080"], protocmp.Transform()); diff != "" {
		t.Errorf("Publish(route update) modified previous snapshot (-want +got):\n%s", diff)
	}

	config.Backends[0].Endpoints[0].Address = "192.0.2.3"
	changedEndpoint := publishTestSnapshot(t, publisher, config)

	assertNamedResourceChanges(t, changedRoute, changedEndpoint, resourcev3.EndpointType, "backend/orders")

	config.Listeners = config.Listeners[1:]
	config.Backends = config.Backends[1:]
	afterDeletion := publishTestSnapshot(t, publisher, config)

	for typeURL, removed := range map[resourcev3.Type]string{
		resourcev3.ListenerType: "listener/8080", resourcev3.RouteType: "route/8080",
		resourcev3.ClusterType: "backend/orders", resourcev3.EndpointType: "backend/orders",
	} {
		if _, exists := afterDeletion.GetResources(typeURL)[removed]; exists {
			t.Errorf("Publish(deletion) retained resource %s/%s", typeURL, removed)
		}

		if _, exists := afterDeletion.GetVersionMap(typeURL)[removed]; exists {
			t.Errorf("Publish(deletion) retained version %s/%s", typeURL, removed)
		}

		if _, exists := changedEndpoint.GetResources(typeURL)[removed]; !exists {
			t.Errorf("Publish(deletion) modified previous snapshot %s/%s", typeURL, removed)
		}

		if changedEndpoint.GetVersionMap(typeURL)[removed] == "" {
			t.Errorf("Publish(deletion) removed previous snapshot version %s/%s", typeURL, removed)
		}

		for name, resource := range afterDeletion.GetResources(typeURL) {
			if resource != changedEndpoint.GetResources(typeURL)[name] {
				t.Errorf("Publish(deletion) replaced unrelated resource %s/%s", typeURL, name)
			}
		}
	}
}

func TestSnapshotResourceOrderDoesNotChangeVersions(t *testing.T) {
	config := twoPortConfig()
	publisher := NewPublisher("", "")
	first := publishTestSnapshot(t, publisher, config)

	slices.Reverse(config.Listeners)
	slices.Reverse(config.Backends)
	reordered := publishTestSnapshot(t, publisher, config)

	if reordered != first {
		t.Error("Publish(reordered independent resources) replaced unchanged snapshot")
	}
}

func TestSnapshotPreservesRoutePriority(t *testing.T) {
	config := twoPortConfig()
	host := &config.Listeners[0].Hosts[0]
	other := host.Routes[0]
	other.Name = "second"
	other.BackendName = "operations"
	host.Routes = append(host.Routes, other)
	publisher := NewPublisher("", "")
	first := publishTestSnapshot(t, publisher, config)

	// 相同路径条件依赖原列表顺序；不得像独立 RDS 资源一样按名称排序。
	slices.Reverse(host.Routes)
	reordered := publishTestSnapshot(t, publisher, config)

	assertNamedResourceChanges(t, first, reordered, resourcev3.RouteType, "route/8080")
}

func TestNewSnapshotRejectsDuplicateResourceNames(t *testing.T) {
	for _, typeURL := range []resourcev3.Type{
		resourcev3.ListenerType, resourcev3.RouteType, resourcev3.ClusterType, resourcev3.EndpointType,
	} {
		t.Run(typeURL, func(t *testing.T) {
			resources, err := buildResources(twoPortConfig())
			if err != nil {
				t.Fatalf("buildResources(two ports) error = %v", err)
			}

			switch typeURL {
			case resourcev3.ListenerType:
				resources.listeners = append(resources.listeners, resources.listeners[0])
			case resourcev3.RouteType:
				resources.routes = append(resources.routes, resources.routes[0])
			case resourcev3.ClusterType:
				resources.clusters = append(resources.clusters, resources.clusters[0])
			case resourcev3.EndpointType:
				resources.endpoints = append(resources.endpoints, resources.endpoints[0])
			}

			if _, err := newSnapshot(resources); err == nil {
				t.Errorf("newSnapshot(duplicate %s name) error = nil, want duplicate rejected", typeURL)
			}
		})
	}
}

func TestPublishDuplicateNamesRetainsPreviousSnapshot(t *testing.T) {
	config := twoPortConfig()
	publisher := NewPublisher("", "")
	first := publishTestSnapshot(t, publisher, config)

	config.Backends = append(config.Backends, config.Backends[0])
	if err := publisher.Publish(t.Context(), config); err == nil {
		t.Fatal("Publish(duplicate Backend) error = nil, want duplicate rejected")
	}

	current, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot(after duplicate rejected) error = %v", err)
	}

	if current != first {
		t.Error("Publish(duplicate Backend) replaced previous snapshot")
	}

	emptyAI := aidataplane.Config{
		FormatVersion:   aidataplane.FormatVersionV1,
		Listeners:       []aidataplane.Listener{},
		Backends:        []aidataplane.Backend{},
		NetworkBackends: []aidataplane.NetworkBackend{},
	}
	if err := publisher.PublishAI(t.Context(), emptyAI); err != nil {
		t.Fatalf("PublishAI(after HTTP candidate rejected) error = %v, want prior HTTP desired state retained", err)
	}

	config.Backends = config.Backends[:2]
	if restored := publishTestSnapshot(t, publisher, config); restored != first {
		t.Error("Publish(valid retry) changed snapshot after duplicate rejected")
	}
}

func TestResourceVersionDependsOnNamedVersions(t *testing.T) {
	first := map[string]string{"route/8080": "a", "route/9090": "b"}
	version := resourceVersion(first)

	if got := resourceVersion(map[string]string{"route/9090": "b", "route/8080": "a"}); got != version {
		t.Errorf("resourceVersion(reordered map) = %q, want %q", got, version)
	}

	for name, changed := range map[string]map[string]string{
		"content": {"route/8080": "changed", "route/9090": "b"},
		"rename":  {"route/8181": "a", "route/9090": "b"},
		"delete":  {"route/8080": "a"},
	} {
		if got := resourceVersion(changed); got == version {
			t.Errorf("resourceVersion(%s) = %q, want different from %q", name, got, version)
		}
	}

	if resourceVersion(map[string]string{"a": "bc"}) == resourceVersion(map[string]string{"ab": "c"}) {
		t.Error("resourceVersion(ambiguous concatenation) collided, want length-delimited names and versions")
	}
}

func twoPortConfig() dataplane.Config {
	config := dataplane.Config{
		Backends: []dataplane.Backend{
			{Name: "orders", Endpoints: []dataplane.Endpoint{{Address: "192.0.2.1", Port: 9000, Weight: 1}}},
			{Name: "operations", Endpoints: []dataplane.Endpoint{{Address: "192.0.2.2", Port: 9000, Weight: 1}}},
		},
	}
	for i, port := range []int32{8080, 9090} {
		backend := config.Backends[i].Name
		config.Listeners = append(config.Listeners, dataplane.Listener{
			Port: port,
			Hosts: []dataplane.Host{{
				Name: "gateway/" + backend,
				Routes: []dataplane.Route{{
					Name: backend, Path: "/orders", PathType: dataplane.PathMatchPrefix, BackendName: backend,
				}},
			}},
		})
	}

	return config
}

func publishTestSnapshot(t *testing.T, publisher *Publisher, config dataplane.Config) cachev3.ResourceSnapshot {
	t.Helper()
	if err := publisher.Publish(t.Context(), config); err != nil {
		t.Fatalf("Publish(configuration) error = %v", err)
	}

	snapshot, err := publisher.cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}

	return snapshot
}

func assertNamedResourceChanges(
	t *testing.T,
	previous, next cachev3.ResourceSnapshot,
	changedType resourcev3.Type,
	changedName string,
) {
	t.Helper()
	for _, typeURL := range []resourcev3.Type{
		resourcev3.ListenerType, resourcev3.RouteType, resourcev3.ClusterType, resourcev3.EndpointType,
	} {
		before := previous.GetResources(typeURL)
		after := next.GetResources(typeURL)
		if len(after) != len(before) {
			t.Errorf("resource count(%s) = %d, want unchanged %d", typeURL, len(after), len(before))
		}

		for name, resource := range before {
			nextResource, exists := after[name]
			if !exists {
				t.Errorf("resource %s/%s disappeared during content update", typeURL, name)
				continue
			}

			changed := typeURL == changedType && name == changedName
			sameResource := resource == nextResource
			sameVersion := previous.GetVersionMap(typeURL)[name] == next.GetVersionMap(typeURL)[name]
			if sameResource == changed || sameVersion == changed {
				t.Errorf("resource %s/%s reused/version equal = %t/%t, want changed = %t", typeURL, name, sameResource, sameVersion, changed)
			}
		}
	}
}
