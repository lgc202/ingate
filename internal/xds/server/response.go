package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	listenerTypeURL = "type.googleapis.com/envoy.config.listener.v3.Listener"
	routeTypeURL    = "type.googleapis.com/envoy.config.route.v3.RouteConfiguration"
	clusterTypeURL  = "type.googleapis.com/envoy.config.cluster.v3.Cluster"
	endpointTypeURL = "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment"
)

type responseBuilder struct {
	store *snapshotStore
}

type snapshotConfig struct {
	Gateway string
	Version string
	Config  targetxds.Config
}

func newResponseBuilder(store *snapshotStore) responseBuilder {
	return responseBuilder{store: store}
}

func (b responseBuilder) Build(request *discoveryv3.DiscoveryRequest) (*discoveryv3.DiscoveryResponse, bool, error) {
	configs, err := b.snapshotConfigs()
	if err != nil {
		return nil, false, err
	}

	var resources []*anypb.Any
	switch request.GetTypeUrl() {
	case listenerTypeURL:
		resources, err = b.buildListeners(configs)
	case routeTypeURL:
		resources, err = b.buildRouteConfigs(configs)
	case clusterTypeURL:
		resources, err = b.buildClusters(configs)
	case endpointTypeURL:
		resources, err = b.buildEndpointAssignments(configs)
	case "":
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("unsupported xds type %q", request.GetTypeUrl())
	}
	if err != nil {
		return nil, false, err
	}

	version := b.responseVersion(configs)
	return &discoveryv3.DiscoveryResponse{
		VersionInfo: version,
		Resources:   b.filterResources(resources, request.GetResourceNames()),
		TypeUrl:     request.GetTypeUrl(),
		Nonce:       version,
	}, true, nil
}

func (b responseBuilder) snapshotConfigs() ([]snapshotConfig, error) {
	snapshots := b.store.List()
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Spec.Gateway < snapshots[j].Spec.Gateway
	})

	configs := make([]snapshotConfig, 0, len(snapshots))
	for _, snapshot := range snapshots {
		var config targetxds.Config
		if err := json.Unmarshal(snapshot.Spec.Config.Raw, &config); err != nil {
			return nil, fmt.Errorf("decode runtime snapshot %q config: %w", snapshot.Name, err)
		}
		configs = append(configs, snapshotConfig{
			Gateway: snapshot.Spec.Gateway,
			Version: snapshot.Spec.Version,
			Config:  config,
		})
	}
	return configs, nil
}

func (b responseBuilder) responseVersion(configs []snapshotConfig) string {
	if len(configs) == 0 {
		return "empty"
	}

	versions := make([]string, 0, len(configs))
	for _, config := range configs {
		versions = append(versions, fmt.Sprintf("%s=%s", config.Gateway, config.Version))
	}
	return strings.Join(versions, ",")
}

func (b responseBuilder) filterResources(resources []*anypb.Any, names []string) []*anypb.Any {
	if len(names) == 0 {
		return resources
	}

	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	filtered := make([]*anypb.Any, 0, len(resources))
	for _, resource := range resources {
		name := b.resourceName(resource)
		if allowed[name] {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

func (b responseBuilder) resourceName(resource *anypb.Any) string {
	switch resource.TypeUrl {
	case listenerTypeURL:
		var listener listenerv3.Listener
		if resource.UnmarshalTo(&listener) == nil {
			return listener.Name
		}
	case routeTypeURL:
		var routeConfig routev3.RouteConfiguration
		if resource.UnmarshalTo(&routeConfig) == nil {
			return routeConfig.Name
		}
	case clusterTypeURL:
		var cluster clusterv3.Cluster
		if resource.UnmarshalTo(&cluster) == nil {
			return cluster.Name
		}
	case endpointTypeURL:
		var assignment endpointv3.ClusterLoadAssignment
		if resource.UnmarshalTo(&assignment) == nil {
			return assignment.ClusterName
		}
	}
	return ""
}

func (b responseBuilder) adsConfigSource() *corev3.ConfigSource {
	return &corev3.ConfigSource{
		ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
			Ads: &corev3.AggregatedConfigSource{},
		},
		ResourceApiVersion: corev3.ApiVersion_V3,
	}
}

func (b responseBuilder) socketAddress(address string, port int) *corev3.Address {
	return &corev3.Address{
		Address: &corev3.Address_SocketAddress{
			SocketAddress: &corev3.SocketAddress{
				Address: address,
				PortSpecifier: &corev3.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		},
	}
}

func (b responseBuilder) mustAny(message proto.Message) *anypb.Any {
	value, err := anypb.New(message)
	if err != nil {
		panic(err)
	}
	return value
}
