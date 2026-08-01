package delivery

import (
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/lgc202/ingate/internal/controller/compiler"
	"google.golang.org/protobuf/proto"
)

// transitionTypeURLs 同时保留 Active 和 Candidate 使用的类型，确保资源删除也会产生待确认的空响应
func transitionTypeURLs(active *publishedConfig, candidate compiler.EnvoyConfig) []string {
	required := make(map[string]bool, len(dynamicTypeURLs()))
	for _, typeURL := range configTypeURLs(candidate) {
		required[typeURL] = true
	}
	if active != nil {
		for _, typeURL := range configTypeURLs(active.config) {
			required[typeURL] = true
		}
	}

	result := make([]string, 0, len(required))
	for _, typeURL := range dynamicTypeURLs() {
		if required[typeURL] {
			result = append(result, typeURL)
		}
	}
	return result
}

func configTypeURLs(value compiler.EnvoyConfig) []string {
	result := []string{resourcev3.ListenerType}
	if len(value.Routes) > 0 {
		result = append(result, resourcev3.RouteType)
	}
	if len(value.Clusters) > 0 {
		result = append(result, resourcev3.ClusterType)
	}
	if len(value.Endpoints) > 0 {
		result = append(result, resourcev3.EndpointType)
	}
	return result
}

func dynamicTypeURLs() []string {
	return []string{
		resourcev3.ListenerType,
		resourcev3.RouteType,
		resourcev3.ClusterType,
		resourcev3.EndpointType,
	}
}

// cloneConfig 隔离编译结果与异步发布过程，避免调用方后续修改 protobuf 对象
func cloneConfig(value compiler.EnvoyConfig) compiler.EnvoyConfig {
	cloned := compiler.EnvoyConfig{
		Listeners: make([]*listenerv3.Listener, 0, len(value.Listeners)),
		Routes:    make([]*routev3.RouteConfiguration, 0, len(value.Routes)),
		Clusters:  make([]*clusterv3.Cluster, 0, len(value.Clusters)),
		Endpoints: make([]*endpointv3.ClusterLoadAssignment, 0, len(value.Endpoints)),
	}
	for _, listener := range value.Listeners {
		cloned.Listeners = append(cloned.Listeners, proto.CloneOf(listener))
	}
	for _, route := range value.Routes {
		cloned.Routes = append(cloned.Routes, proto.CloneOf(route))
	}
	for _, cluster := range value.Clusters {
		cloned.Clusters = append(cloned.Clusters, proto.CloneOf(cluster))
	}
	for _, endpoint := range value.Endpoints {
		cloned.Endpoints = append(cloned.Endpoints, proto.CloneOf(endpoint))
	}
	return cloned
}

func configsEqual(a, b compiler.EnvoyConfig) bool {
	if len(a.Listeners) != len(b.Listeners) || len(a.Routes) != len(b.Routes) ||
		len(a.Clusters) != len(b.Clusters) || len(a.Endpoints) != len(b.Endpoints) {
		return false
	}
	for i := range a.Listeners {
		if !proto.Equal(a.Listeners[i], b.Listeners[i]) {
			return false
		}
	}
	for i := range a.Routes {
		if !proto.Equal(a.Routes[i], b.Routes[i]) {
			return false
		}
	}
	for i := range a.Clusters {
		if !proto.Equal(a.Clusters[i], b.Clusters[i]) {
			return false
		}
	}
	for i := range a.Endpoints {
		if !proto.Equal(a.Endpoints[i], b.Endpoints[i]) {
			return false
		}
	}
	return true
}
