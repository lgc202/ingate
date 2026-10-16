// Package envoy 把已接纳的共享数据面配置转换为 Envoy xDS 资源。
package envoy

import (
	"cmp"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
)

const (
	listenerAddress  = "0.0.0.0"
	hcmFilterName    = "envoy.filters.network.http_connection_manager"
	routerFilterName = "envoy.filters.http.router"
)

// resourceSet 只在 Envoy 适配层内使用，不向 Controller 暴露协议类型。
type resourceSet struct {
	listeners []*listenerv3.Listener
	routes    []*routev3.RouteConfiguration
	clusters  []*clusterv3.Cluster
	endpoints []*endpointv3.ClusterLoadAssignment
	// 模型入口在发布时合入 HTTP 拥有的公共 VirtualHost，不另建重复 Listener。
	modelRoutes map[modelEntryKey]*routev3.Route
}

func buildResources(config dataplane.Config) (resourceSet, error) {
	var resources resourceSet
	for _, listener := range config.Listeners {
		routeConfig, err := newRouteConfiguration(listener)
		if err != nil {
			return resourceSet{}, err
		}

		resources.routes = append(resources.routes, routeConfig)

		xdsListener, err := newListener(listener.Port)
		if err != nil {
			return resourceSet{}, err
		}

		resources.listeners = append(resources.listeners, xdsListener)
	}

	for _, backend := range config.Backends {
		cluster, assignment, err := newCluster(backend)
		if err != nil {
			return resourceSet{}, err
		}

		resources.clusters = append(resources.clusters, cluster)
		if cluster.GetType() == clusterv3.Cluster_EDS {
			resources.endpoints = append(resources.endpoints, assignment)
		}
	}

	return resources, nil
}

func newRouteConfiguration(listener dataplane.Listener) (*routev3.RouteConfiguration, error) {
	config := &routev3.RouteConfiguration{
		Name:                     routeName(listener.Port),
		IgnorePortInHostMatching: true,
	}
	for _, host := range listener.Hosts {
		virtualHost, err := newVirtualHost(host)
		if err != nil {
			return nil, err
		}

		config.VirtualHosts = append(config.VirtualHosts, virtualHost)
	}

	if err := config.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate route configuration %q: %w", config.Name, err)
	}

	return config, nil
}

func newVirtualHost(host dataplane.Host) (*routev3.VirtualHost, error) {
	domain := host.Hostname
	if domain == "" {
		domain = "*"
	}

	virtualHost := &routev3.VirtualHost{Name: host.Name, Domains: []string{domain}}

	routes := slices.Clone(host.Routes)
	slices.SortStableFunc(routes, compareRoutes)
	for _, route := range routes {
		match, err := newRouteMatch(route)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", route.Name, err)
		}

		xdsRoute := &routev3.Route{
			Name:  route.Name,
			Match: match,
			Action: &routev3.Route_Route{Route: &routev3.RouteAction{
				ClusterSpecifier: &routev3.RouteAction_Cluster{Cluster: clusterName(route.BackendName)},
			}},
		}
		if route.BackendName == "" {
			xdsRoute.Action = &routev3.Route_DirectResponse{DirectResponse: &routev3.DirectResponseAction{Status: 500}}
		}

		virtualHost.Routes = append(virtualHost.Routes, xdsRoute)
	}

	return virtualHost, nil
}

// Envoy 按数组顺序选择首个匹配项；平局保留 Controller 的跨资源和规则列表排序。
func compareRoutes(a, b dataplane.Route) int {
	if a.PathType != b.PathType {
		if a.PathType == dataplane.PathMatchExact {
			return -1
		}

		if b.PathType == dataplane.PathMatchExact {
			return 1
		}

		return cmp.Compare(a.PathType, b.PathType)
	}

	if a.PathType == dataplane.PathMatchPrefix && len(a.Path) != len(b.Path) {
		return cmp.Compare(len(b.Path), len(a.Path))
	}

	return 0
}

func newRouteMatch(route dataplane.Route) (*routev3.RouteMatch, error) {
	match := &routev3.RouteMatch{}

	switch route.PathType {
	case dataplane.PathMatchExact:
		match.PathSpecifier = &routev3.RouteMatch_Path{Path: route.Path}
	case dataplane.PathMatchPrefix:
		if route.Path == "/" {
			match.PathSpecifier = &routev3.RouteMatch_Prefix{Prefix: route.Path}
		} else {
			match.PathSpecifier = &routev3.RouteMatch_PathSeparatedPrefix{PathSeparatedPrefix: route.Path}
		}
	default:
		return nil, fmt.Errorf("unsupported path match type %q", route.PathType)
	}

	return match, nil
}

func newListener(port int32) (*listenerv3.Listener, error) {
	routerConfig, err := anypb.New(&routerv3.Router{})
	if err != nil {
		return nil, fmt.Errorf("marshal router filter: %w", err)
	}

	hcm := &hcmv3.HttpConnectionManager{
		StatPrefix: "listener_" + strconv.FormatInt(int64(port), 10),
		RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{Rds: &hcmv3.Rds{
			ConfigSource:    adsConfigSource(),
			RouteConfigName: routeName(port),
		}},
		HttpFilters: []*hcmv3.HttpFilter{{
			Name:       routerFilterName,
			ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: routerConfig},
		}},
	}
	if err := hcm.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate HTTP connection manager for port %d: %w", port, err)
	}

	hcmConfig, err := anypb.New(hcm)
	if err != nil {
		return nil, fmt.Errorf("marshal HTTP connection manager: %w", err)
	}

	hcmFilter := &listenerv3.Filter{
		Name:       hcmFilterName,
		ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: hcmConfig},
	}
	listener := &listenerv3.Listener{
		Name:    "listener/" + strconv.FormatInt(int64(port), 10),
		Address: socketAddress(listenerAddress, port),
		FilterChains: []*listenerv3.FilterChain{{
			Filters: []*listenerv3.Filter{hcmFilter},
		}},
	}
	if err := listener.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate listener %q: %w", listener.Name, err)
	}

	return listener, nil
}

func newCluster(backend dataplane.Backend) (*clusterv3.Cluster, *endpointv3.ClusterLoadAssignment, error) {
	name := clusterName(backend.Name)
	assignment, hasDNSName := newEndpointAssignment(name, backend.Endpoints)
	cluster := &clusterv3.Cluster{
		Name:           name,
		ConnectTimeout: durationpb.New(5 * time.Second),
		LbPolicy:       clusterv3.Cluster_ROUND_ROBIN,
	}
	if hasDNSName {
		// EDS 不负责普通 DNS 解析；含域名的后端交由 Envoy 的 DNS Cluster 解析。
		cluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_STRICT_DNS}
		cluster.LoadAssignment = assignment
	} else {
		cluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_EDS}
		cluster.EdsClusterConfig = &clusterv3.Cluster_EdsClusterConfig{
			EdsConfig:   adsConfigSource(),
			ServiceName: name,
		}
	}

	if err := cluster.ValidateAll(); err != nil {
		return nil, nil, fmt.Errorf("validate cluster %q: %w", name, err)
	}

	if err := assignment.ValidateAll(); err != nil {
		return nil, nil, fmt.Errorf("validate endpoints for %q: %w", name, err)
	}

	return cluster, assignment, nil
}

// newEndpointAssignment 克隆后排序端点并识别 DNS；完整校验由 newCluster 按原顺序执行。
func newEndpointAssignment(name string, endpoints []dataplane.Endpoint) (*endpointv3.ClusterLoadAssignment, bool) {
	ordered := slices.Clone(endpoints)
	slices.SortFunc(ordered, func(a, b dataplane.Endpoint) int {
		if n := cmp.Compare(a.Address, b.Address); n != 0 {
			return n
		}

		if n := cmp.Compare(a.Port, b.Port); n != 0 {
			return n
		}

		return cmp.Compare(a.Weight, b.Weight)
	})

	locality := &endpointv3.LocalityLbEndpoints{}
	hasDNSName := false
	for _, endpoint := range ordered {
		if _, err := netip.ParseAddr(endpoint.Address); err != nil {
			hasDNSName = true
		}

		locality.LbEndpoints = append(locality.LbEndpoints, &endpointv3.LbEndpoint{
			HostIdentifier: &endpointv3.LbEndpoint_Endpoint{Endpoint: &endpointv3.Endpoint{
				Address: socketAddress(endpoint.Address, endpoint.Port),
			}},
			LoadBalancingWeight: wrapperspb.UInt32(uint32(endpoint.Weight)),
		})
	}

	return &endpointv3.ClusterLoadAssignment{
		ClusterName: name,
		Endpoints:   []*endpointv3.LocalityLbEndpoints{locality},
	}, hasDNSName
}

func adsConfigSource() *corev3.ConfigSource {
	return &corev3.ConfigSource{
		ConfigSourceSpecifier: &corev3.ConfigSource_Ads{Ads: &corev3.AggregatedConfigSource{}},
		ResourceApiVersion:    corev3.ApiVersion_V3,
	}
}

func socketAddress(address string, port int32) *corev3.Address {
	return &corev3.Address{
		Address: &corev3.Address_SocketAddress{
			SocketAddress: &corev3.SocketAddress{
				Address:       address,
				PortSpecifier: &corev3.SocketAddress_PortValue{PortValue: uint32(port)},
			},
		},
	}
}

func routeName(port int32) string {
	return "route/" + strconv.FormatInt(int64(port), 10)
}

func clusterName(backend string) string {
	return "backend/" + backend
}
