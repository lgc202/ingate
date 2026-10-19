package xds

import (
	"testing"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

// twoPortConfig 为分发测试提供两个独立 RDS/EDS 资源，不依赖 Envoy 转换器的私有结构。
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

	cache, ok := publisher.envoy.DiscoveryCache().(cachev3.SnapshotCache)
	if !ok {
		t.Fatal("Envoy DiscoveryCache() is not a SnapshotCache")
	}

	snapshot, err := cache.GetSnapshot(sharedDataPlaneCluster)
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}

	return snapshot
}
