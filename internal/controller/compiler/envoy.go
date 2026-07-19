package compiler

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	cachetypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/proto"
)

const versionPrefix = "ingate/"

type versionRecord struct {
	typeURL string
	name    string
	message proto.Message
}

// Snapshot 将 EnvoyConfig 转换为同时包含 LDS、RDS、CDS 和 EDS 的一致快照
func (c EnvoyConfig) Snapshot(version string) (*cachev3.Snapshot, error) {
	if version == "" {
		return nil, errors.New("snapshot version is required")
	}

	listeners := make([]cachetypes.Resource, 0, len(c.Listeners))
	for _, listener := range c.Listeners {
		if listener == nil {
			return nil, errors.New("listener resource is nil")
		}
		if err := listener.ValidateAll(); err != nil {
			return nil, fmt.Errorf("validate listener %q: %w", listener.GetName(), err)
		}
		listeners = append(listeners, listener)
	}
	routes := make([]cachetypes.Resource, 0, len(c.Routes))
	for _, route := range c.Routes {
		if route == nil {
			return nil, errors.New("route resource is nil")
		}
		if err := route.ValidateAll(); err != nil {
			return nil, fmt.Errorf("validate route %q: %w", route.GetName(), err)
		}
		routes = append(routes, route)
	}
	clusters := make([]cachetypes.Resource, 0, len(c.Clusters))
	for _, cluster := range c.Clusters {
		if cluster == nil {
			return nil, errors.New("cluster resource is nil")
		}
		if err := cluster.ValidateAll(); err != nil {
			return nil, fmt.Errorf("validate cluster %q: %w", cluster.GetName(), err)
		}
		clusters = append(clusters, cluster)
	}
	endpoints := make([]cachetypes.Resource, 0, len(c.Endpoints))
	for _, endpoint := range c.Endpoints {
		if endpoint == nil {
			return nil, errors.New("endpoint resource is nil")
		}
		if err := endpoint.ValidateAll(); err != nil {
			return nil, fmt.Errorf("validate endpoint %q: %w", endpoint.GetClusterName(), err)
		}
		endpoints = append(endpoints, endpoint)
	}

	snapshot, err := cachev3.NewSnapshot(version, map[resourcev3.Type][]cachetypes.Resource{
		resourcev3.ListenerType: listeners,
		resourcev3.RouteType:    routes,
		resourcev3.ClusterType:  clusters,
		resourcev3.EndpointType: endpoints,
	})
	if err != nil {
		return nil, fmt.Errorf("create xDS snapshot: %w", err)
	}
	if err := snapshot.Consistent(); err != nil {
		return nil, fmt.Errorf("check xDS snapshot consistency: %w", err)
	}
	return snapshot, nil
}

func (c EnvoyConfig) version() (string, error) {
	// 版本只由排序后的资源类型、名字和确定性 protobuf bytes 决定，不依赖输入集合顺序
	records := make([]versionRecord, 0, len(c.Listeners)+len(c.Routes)+len(c.Clusters)+len(c.Endpoints))
	for _, listener := range c.Listeners {
		records = append(records, versionRecord{
			typeURL: resourcev3.ListenerType,
			name:    listener.GetName(),
			message: listener,
		})
	}
	for _, route := range c.Routes {
		records = append(records, versionRecord{
			typeURL: resourcev3.RouteType,
			name:    route.GetName(),
			message: route,
		})
	}
	for _, cluster := range c.Clusters {
		records = append(records, versionRecord{
			typeURL: resourcev3.ClusterType,
			name:    cluster.GetName(),
			message: cluster,
		})
	}
	for _, endpoint := range c.Endpoints {
		records = append(records, versionRecord{
			typeURL: resourcev3.EndpointType,
			name:    endpoint.GetClusterName(),
			message: endpoint,
		})
	}
	slices.SortFunc(records, func(a, b versionRecord) int {
		if a.typeURL != b.typeURL {
			return cmp.Compare(a.typeURL, b.typeURL)
		}
		return cmp.Compare(a.name, b.name)
	})

	data := make([]byte, 0)
	marshal := proto.MarshalOptions{Deterministic: true}
	for _, record := range records {
		payload, err := marshal.Marshal(record.message)
		if err != nil {
			return "", fmt.Errorf("marshal %s %q: %w", record.typeURL, record.name, err)
		}
		data = appendVersionField(data, []byte(record.typeURL))
		data = appendVersionField(data, []byte(record.name))
		data = appendVersionField(data, payload)
	}

	sum := sha256.Sum256(data)
	return versionPrefix + hex.EncodeToString(sum[:]), nil
}

func appendVersionField(data, field []byte) []byte {
	data = binary.AppendUvarint(data, uint64(len(field)))
	return append(data, field...)
}

func adsConfigSource() *corev3.ConfigSource {
	return &corev3.ConfigSource{
		ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
			Ads: &corev3.AggregatedConfigSource{},
		},
		ResourceApiVersion: corev3.ApiVersion_V3,
	}
}
