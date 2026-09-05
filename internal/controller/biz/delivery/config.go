package delivery

import (
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/samber/lo"
	"google.golang.org/protobuf/proto"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
)

// transitionTypeURLs 同时保留 Active 和 Candidate 使用的类型，
// 确保资源删除也会产生待确认的空响应。
func transitionTypeURLs(active *publishedConfig, candidate compiler.EnvoyConfig) []string {
	typeURLs := dynamicTypeURLs()
	required := make(map[string]bool, len(typeURLs))
	for _, typeURL := range configTypeURLs(candidate) {
		required[typeURL] = true
	}
	if active != nil {
		for _, typeURL := range configTypeURLs(active.config) {
			required[typeURL] = true
		}
	}

	result := make([]string, 0, len(required))
	for _, typeURL := range typeURLs {
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

// cloneConfig 隔离编译结果与异步发布过程，避免调用方后续修改 protobuf 对象。
func cloneConfig(value compiler.EnvoyConfig) compiler.EnvoyConfig {
	return compiler.EnvoyConfig{
		Listeners: lo.Map(value.Listeners, func(listener *listenerv3.Listener, _ int) *listenerv3.Listener {
			return proto.CloneOf(listener)
		}),
		Routes: lo.Map(value.Routes, func(route *routev3.RouteConfiguration, _ int) *routev3.RouteConfiguration {
			return proto.CloneOf(route)
		}),
		Clusters: lo.Map(value.Clusters, func(cluster *clusterv3.Cluster, _ int) *clusterv3.Cluster {
			return proto.CloneOf(cluster)
		}),
		Endpoints: lo.Map(value.Endpoints, func(endpoint *endpointv3.ClusterLoadAssignment, _ int) *endpointv3.ClusterLoadAssignment {
			return proto.CloneOf(endpoint)
		}),
	}
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
