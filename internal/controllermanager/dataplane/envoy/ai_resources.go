package envoy

import (
	"fmt"
	"net"
	"slices"
	"strconv"

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

func newAICluster(
	target string,
	network aidataplane.NetworkBackend,
) (*clusterv3.Cluster, *endpointv3.ClusterLoadAssignment, error) {
	addresses := make([]dataplane.Endpoint, len(network.Endpoints))
	for i, endpoint := range network.Endpoints {
		addresses[i] = dataplane.Endpoint{Address: endpoint.Address, Port: endpoint.Port, Weight: endpoint.Weight}
	}

	var tls *dataplane.BackendTLSSettings
	if network.TLS != nil {
		tls = &dataplane.BackendTLSSettings{
			SNI: network.TLS.SNI,
		}
	}

	cluster, assignment, err := newCluster(dataplane.Backend{Name: target, Endpoints: addresses, TLS: tls})
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

func newAIEgressRoutes(entries []*routev3.Route) (*routev3.RouteConfiguration, error) {
	// 未知、缺失或已撤销目标仍明确拒绝，不回退到普通 HTTP 或其他 AI 后端。
	routes := append(slices.Clone(entries), &routev3.Route{
		Name:   aiEgressName + "/unavailable",
		Match:  &routev3.RouteMatch{PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: "/"}},
		Action: &routev3.Route_DirectResponse{DirectResponse: &routev3.DirectResponseAction{Status: 503}},
	})
	config := &routev3.RouteConfiguration{
		Name: aiEgressName,
		VirtualHosts: []*routev3.VirtualHost{{
			Name: aiEgressName, Domains: []string{"*"}, Routes: routes,
		}},
	}
	if err := config.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate AI egress routes: %w", err)
	}

	return config, nil
}

// newAIEgressRoute 按受信任目标 Header 和 POST 选择静态 Cluster，模型与 MCP 共用网络出口。
func newAIEgressRoute(target, cluster string) *routev3.Route {
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
