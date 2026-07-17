package lastgood

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"slices"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/lgc202/ingate/internal/envoy/config"
	"google.golang.org/protobuf/proto"
)

// NewRecord 将完整 Envoy 配置编码为可持久化记录
func NewRecord(version string, value config.Config, generatedAt time.Time) (Record, error) {
	if version == "" {
		return Record{}, fmt.Errorf("%w: version is empty", ErrCorrupt)
	}

	listeners, err := marshalResources(value.Listeners, func(resource *listenerv3.Listener) string {
		return resource.GetName()
	})
	if err != nil {
		return Record{}, fmt.Errorf("marshal listeners: %w", err)
	}
	routes, err := marshalResources(value.Routes, func(resource *routev3.RouteConfiguration) string {
		return resource.GetName()
	})
	if err != nil {
		return Record{}, fmt.Errorf("marshal routes: %w", err)
	}
	clusters, err := marshalResources(value.Clusters, func(resource *clusterv3.Cluster) string {
		return resource.GetName()
	})
	if err != nil {
		return Record{}, fmt.Errorf("marshal clusters: %w", err)
	}
	endpoints, err := marshalResources(value.Endpoints, func(resource *endpointv3.ClusterLoadAssignment) string {
		return resource.GetClusterName()
	})
	if err != nil {
		return Record{}, fmt.Errorf("marshal endpoints: %w", err)
	}

	record := Record{
		SchemaVersion: SchemaVersion,
		Version:       version,
		GeneratedAt:   generatedAt.UTC(),
		Listeners:     listeners,
		Routes:        routes,
		Clusters:      clusters,
		Endpoints:     endpoints,
	}
	record.ContentHash = contentHash(record)
	if _, err := record.Config(); err != nil {
		return Record{}, err
	}
	return record, nil
}

// Config 校验记录并恢复完整 Envoy 配置
func (r Record) Config() (config.Config, error) {
	if r.SchemaVersion != SchemaVersion {
		return config.Config{}, fmt.Errorf("%w: schema version %d", ErrIncompatible, r.SchemaVersion)
	}
	if r.Version == "" {
		return config.Config{}, fmt.Errorf("%w: version is empty", ErrCorrupt)
	}
	if r.ContentHash == "" || r.ContentHash != contentHash(r) {
		return config.Config{}, fmt.Errorf("%w: content hash mismatch", ErrCorrupt)
	}

	listeners, err := unmarshalListeners(r.Listeners)
	if err != nil {
		return config.Config{}, err
	}
	routes, err := unmarshalRoutes(r.Routes)
	if err != nil {
		return config.Config{}, err
	}
	clusters, err := unmarshalClusters(r.Clusters)
	if err != nil {
		return config.Config{}, err
	}
	endpoints, err := unmarshalEndpoints(r.Endpoints)
	if err != nil {
		return config.Config{}, err
	}

	value := config.Config{
		Listeners: listeners,
		Routes:    routes,
		Clusters:  clusters,
		Endpoints: endpoints,
	}
	if _, err := value.Snapshot(r.Version); err != nil {
		return config.Config{}, fmt.Errorf("%w: inconsistent snapshot: %v", ErrCorrupt, err)
	}
	return value, nil
}

func marshalResources[T proto.Message](resources []T, name func(T) string) ([][]byte, error) {
	ordered := slices.Clone(resources)
	slices.SortFunc(ordered, func(a, b T) int {
		return cmp.Compare(name(a), name(b))
	})

	result := make([][]byte, 0, len(ordered))
	previous := ""
	for i, resource := range ordered {
		resourceName := name(resource)
		if resourceName == "" {
			return nil, fmt.Errorf("resource %d has an empty name", i)
		}
		if i > 0 && resourceName == previous {
			return nil, fmt.Errorf("duplicate resource name %q", resourceName)
		}
		data, err := proto.MarshalOptions{Deterministic: true}.Marshal(resource)
		if err != nil {
			return nil, fmt.Errorf("marshal resource %q: %w", resourceName, err)
		}
		result = append(result, data)
		previous = resourceName
	}
	return result, nil
}

func unmarshalListeners(resources [][]byte) ([]*listenerv3.Listener, error) {
	result := make([]*listenerv3.Listener, 0, len(resources))
	for i, data := range resources {
		resource := &listenerv3.Listener{}
		if err := proto.Unmarshal(data, resource); err != nil {
			return nil, fmt.Errorf("%w: decode listener %d: %v", ErrCorrupt, i, err)
		}
		result = append(result, resource)
	}
	if err := sortAndCheckNames(result, func(resource *listenerv3.Listener) string { return resource.GetName() }); err != nil {
		return nil, err
	}
	return result, nil
}

func unmarshalRoutes(resources [][]byte) ([]*routev3.RouteConfiguration, error) {
	result := make([]*routev3.RouteConfiguration, 0, len(resources))
	for i, data := range resources {
		resource := &routev3.RouteConfiguration{}
		if err := proto.Unmarshal(data, resource); err != nil {
			return nil, fmt.Errorf("%w: decode route %d: %v", ErrCorrupt, i, err)
		}
		result = append(result, resource)
	}
	if err := sortAndCheckNames(result, func(resource *routev3.RouteConfiguration) string { return resource.GetName() }); err != nil {
		return nil, err
	}
	return result, nil
}

func unmarshalClusters(resources [][]byte) ([]*clusterv3.Cluster, error) {
	result := make([]*clusterv3.Cluster, 0, len(resources))
	for i, data := range resources {
		resource := &clusterv3.Cluster{}
		if err := proto.Unmarshal(data, resource); err != nil {
			return nil, fmt.Errorf("%w: decode cluster %d: %v", ErrCorrupt, i, err)
		}
		result = append(result, resource)
	}
	if err := sortAndCheckNames(result, func(resource *clusterv3.Cluster) string { return resource.GetName() }); err != nil {
		return nil, err
	}
	return result, nil
}

func unmarshalEndpoints(resources [][]byte) ([]*endpointv3.ClusterLoadAssignment, error) {
	result := make([]*endpointv3.ClusterLoadAssignment, 0, len(resources))
	for i, data := range resources {
		resource := &endpointv3.ClusterLoadAssignment{}
		if err := proto.Unmarshal(data, resource); err != nil {
			return nil, fmt.Errorf("%w: decode endpoint %d: %v", ErrCorrupt, i, err)
		}
		result = append(result, resource)
	}
	if err := sortAndCheckNames(result, func(resource *endpointv3.ClusterLoadAssignment) string { return resource.GetClusterName() }); err != nil {
		return nil, err
	}
	return result, nil
}

func sortAndCheckNames[T any](resources []T, name func(T) string) error {
	slices.SortFunc(resources, func(a, b T) int {
		return cmp.Compare(name(a), name(b))
	})
	previous := ""
	for i, resource := range resources {
		resourceName := name(resource)
		if resourceName == "" {
			return fmt.Errorf("%w: resource %d has an empty name", ErrCorrupt, i)
		}
		if i > 0 && resourceName == previous {
			return fmt.Errorf("%w: duplicate resource name %q", ErrCorrupt, resourceName)
		}
		previous = resourceName
	}
	return nil
}

func contentHash(record Record) string {
	hash := sha256.New()
	writeHashResources(hash, "listeners", record.Listeners)
	writeHashResources(hash, "routes", record.Routes)
	writeHashResources(hash, "clusters", record.Clusters)
	writeHashResources(hash, "endpoints", record.Endpoints)
	return hex.EncodeToString(hash.Sum(nil))
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func writeHashResources(hash hashWriter, resourceType string, resources [][]byte) {
	writeHashBytes(hash, []byte(resourceType))
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(resources)))
	_, _ = hash.Write(count[:])
	for _, resource := range resources {
		writeHashBytes(hash, resource)
	}
}

func writeHashBytes(hash hashWriter, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = hash.Write(size[:])
	_, _ = hash.Write(value)
}
