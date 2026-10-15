package envoy

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

const aiEgressName = "ai/egress"

// buildAIResources 编译模型入口和已被模型路由引用的出口，不开放 MCP 接口。
// 出口 Header 精确匹配静态 Cluster，不使用允许请求选择任意 Cluster 的 cluster_header。
func buildAIResources(config aidataplane.Config, ingressSocket, egressSocket string) (resourceSet, error) {
	if err := aidataplane.Validate(config); err != nil {
		return resourceSet{}, fmt.Errorf("validate AI configuration: %w", err)
	}

	resources := resourceSet{modelRoutes: make(map[modelEntryKey]*routev3.Route)}
	used := make(map[string]bool)
	for _, listener := range config.Listeners {
		for _, host := range listener.Hosts {
			if len(host.ModelRoutes) != 0 {
				key := modelEntryKey{routeName: routeName(listener.Port), hostName: host.Name}
				resources.modelRoutes[key] = newAIIngressRoute(listener.Port, host.Name)
			}
			for _, route := range host.ModelRoutes {
				if *route.BackendName != "" {
					used[*route.BackendName] = true
				}
			}
		}
	}

	if len(resources.modelRoutes) != 0 {
		if !filepath.IsAbs(ingressSocket) || strings.ContainsRune(ingressSocket, 0) {
			return resourceSet{}, errors.New("AI ingress socket must be an absolute path without NUL")
		}
		if filepath.Clean(ingressSocket) == filepath.Clean(egressSocket) {
			return resourceSet{}, errors.New("AI ingress and egress sockets must differ")
		}

		cluster, err := newAIIngressCluster(ingressSocket)
		if err != nil {
			return resourceSet{}, err
		}
		resources.clusters = append(resources.clusters, cluster)
	}
	if len(used) == 0 {
		return resources, nil
	}
	if !filepath.IsAbs(egressSocket) || strings.ContainsRune(egressSocket, 0) {
		return resourceSet{}, errors.New("AI egress socket must be an absolute path without NUL")
	}

	networks := make(map[string]aidataplane.NetworkBackend, len(config.NetworkBackends))
	for _, network := range config.NetworkBackends {
		networks[network.Name] = network
	}
	host := &routev3.VirtualHost{Name: aiEgressName, Domains: []string{"*"}}
	for _, backend := range config.Backends {
		if !used[backend.Name] {
			continue
		}

		network := networks[backend.NetworkBackendName]
		target, err := aidataplane.ModelTargetName(backend, network)
		if err != nil {
			return resourceSet{}, err
		}
		cluster, assignment, err := newAICluster(target, network.Endpoints)
		if err != nil {
			return resourceSet{}, err
		}
		resources.clusters = append(resources.clusters, cluster)
		if cluster.GetType() == clusterv3.Cluster_EDS {
			resources.endpoints = append(resources.endpoints, assignment)
		}
		host.Routes = append(host.Routes, newAIModelRoute(target, cluster.Name))
	}
	// 未知、缺失或已撤销目标明确拒绝，不能命中普通 HTTP 或其他模型后端。
	host.Routes = append(host.Routes, &routev3.Route{
		Name:   aiEgressName + "/unavailable",
		Match:  &routev3.RouteMatch{PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: "/"}},
		Action: &routev3.Route_DirectResponse{DirectResponse: &routev3.DirectResponseAction{Status: 503}},
	})
	routes := &routev3.RouteConfiguration{Name: aiEgressName, VirtualHosts: []*routev3.VirtualHost{host}}
	if err := routes.ValidateAll(); err != nil {
		return resourceSet{}, fmt.Errorf("validate AI egress routes: %w", err)
	}
	listener, err := newAIListener(egressSocket)
	if err != nil {
		return resourceSet{}, err
	}
	resources.routes = append(resources.routes, routes)
	resources.listeners = append(resources.listeners, listener)

	return resources, nil
}

func newAICluster(
	target string,
	endpoints []aidataplane.Endpoint,
) (*clusterv3.Cluster, *endpointv3.ClusterLoadAssignment, error) {
	addresses := make([]dataplane.Endpoint, len(endpoints))
	for i, endpoint := range endpoints {
		addresses[i] = dataplane.Endpoint{Address: endpoint.Address, Port: endpoint.Port, Weight: endpoint.Weight}
	}
	cluster, assignment, err := newCluster(dataplane.Backend{Name: target, Endpoints: addresses})
	if err != nil {
		return nil, nil, err
	}

	// Envoy 按实际选中的端点生成 Host，包括 IP、IPv6 与非默认端口；不采用客户端 Host。
	for _, locality := range assignment.Endpoints {
		for _, endpoint := range locality.LbEndpoints {
			address := endpoint.GetEndpoint().GetAddress().GetSocketAddress()
			port := strconv.FormatUint(uint64(address.GetPortValue()), 10)
			endpoint.GetEndpoint().Hostname = net.JoinHostPort(address.Address, port)
		}
	}
	if err := cluster.ValidateAll(); err != nil {
		return nil, nil, fmt.Errorf("validate AI cluster %q: %w", target, err)
	}
	if err := assignment.ValidateAll(); err != nil {
		return nil, nil, fmt.Errorf("validate AI endpoints %q: %w", target, err)
	}

	return cluster, assignment, nil
}

func newAIModelRoute(target, cluster string) *routev3.Route {
	return &routev3.Route{
		Name: target,
		Match: &routev3.RouteMatch{
			PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: "/"},
			Headers: []*routev3.HeaderMatcher{
				exactAIHeader(aidataplane.TargetHeader, target),
				exactAIHeader(":method", "POST"),
			},
		},
		Action: &routev3.Route_Route{Route: &routev3.RouteAction{
			ClusterSpecifier:     &routev3.RouteAction_Cluster{Cluster: cluster},
			HostRewriteSpecifier: &routev3.RouteAction_AutoHostRewrite{AutoHostRewrite: wrapperspb.Bool(true)},
			// 不让默认整体请求超时截断 SSE；运行时空闲/排空边界仍需组件验证。
			Timeout: durationpb.New(0),
		}},
		RequestHeadersToRemove: []string{aidataplane.TargetHeader},
	}
}

func newAIListener(socket string) (*listenerv3.Listener, error) {
	router, err := anypb.New(&routerv3.Router{})
	if err != nil {
		return nil, fmt.Errorf("marshal AI egress router: %w", err)
	}
	hcm, err := anypb.New(&hcmv3.HttpConnectionManager{
		StatPrefix: "ai_egress",
		RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{Rds: &hcmv3.Rds{
			ConfigSource: adsConfigSource(), RouteConfigName: aiEgressName,
		}},
		HttpFilters: []*hcmv3.HttpFilter{{
			Name: routerFilterName, ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: router},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal AI egress connection manager: %w", err)
	}
	listener := &listenerv3.Listener{
		Name:    aiEgressName,
		Address: &corev3.Address{Address: &corev3.Address_Pipe{Pipe: &corev3.Pipe{Path: socket, Mode: 0o600}}},
		FilterChains: []*listenerv3.FilterChain{{Filters: []*listenerv3.Filter{{
			Name: hcmFilterName, ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: hcm},
		}}}},
	}
	if err := listener.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate AI egress listener: %w", err)
	}

	return listener, nil
}

func exactAIHeader(name, value string) *routev3.HeaderMatcher {
	return &routev3.HeaderMatcher{
		Name: name,
		HeaderMatchSpecifier: &routev3.HeaderMatcher_StringMatch{StringMatch: &matcherv3.StringMatcher{
			MatchPattern: &matcherv3.StringMatcher_Exact{Exact: value},
		}},
	}
}
