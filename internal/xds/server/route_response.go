package server

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	defaultRoutePrefix       = "/"
	retryOnUpstreamTransient = "connect-failure,refused-stream,reset,5xx"
	routeNamePrefix          = "ingate-route"
)

type routeConfigGroup struct {
	virtualHosts []targetxds.VirtualHost
}

func (b responseBuilder) buildRouteConfigs(configs []snapshotConfig) ([]*anypb.Any, error) {
	resources := make([]*anypb.Any, 0)
	groups := map[listenerGroupKey]routeConfigGroup{}
	unlinked := map[string]routeConfigGroup{}

	for _, config := range configs {
		listenerKeys := routeConfigListenerKeys(config.Config)
		for _, routeConfig := range config.Config.RouteConfigs {
			if key, ok := listenerKeys[routeConfig.Name]; ok {
				group := groups[key]
				group.virtualHosts = append(group.virtualHosts, routeConfig.VirtualHosts...)
				groups[key] = group
				continue
			}
			group := unlinked[routeConfig.Name]
			group.virtualHosts = append(group.virtualHosts, routeConfig.VirtualHosts...)
			unlinked[routeConfig.Name] = group
		}
	}

	keys := map[listenerGroupKey]struct{}{}
	for key := range groups {
		keys[key] = struct{}{}
	}
	for _, key := range sortedListenerKeys(keys) {
		group := groups[key]
		resource, err := b.buildRouteConfig(listenerRouteConfigName(key), group.virtualHosts)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}

	names := make([]string, 0, len(unlinked))
	for name := range unlinked {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		group := unlinked[name]
		resource, err := b.buildRouteConfig(name, group.virtualHosts)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (b responseBuilder) buildRouteConfig(name string, virtualHosts []targetxds.VirtualHost) (*anypb.Any, error) {
	virtualHosts = mergeVirtualHosts(virtualHosts)
	resources := make([]*routev3.VirtualHost, 0, len(virtualHosts))
	for _, virtualHost := range virtualHosts {
		routes, err := b.buildRoutes(virtualHost.Routes)
		if err != nil {
			return nil, err
		}
		resources = append(resources, &routev3.VirtualHost{
			Name:    virtualHost.Name,
			Domains: virtualHost.Domains,
			Routes:  routes,
		})
	}

	return anypb.New(&routev3.RouteConfiguration{
		Name:         name,
		VirtualHosts: resources,
	})
}

func mergeVirtualHosts(virtualHosts []targetxds.VirtualHost) []targetxds.VirtualHost {
	result := make([]targetxds.VirtualHost, 0, len(virtualHosts))
	indexes := make(map[string]int, len(virtualHosts))
	for _, virtualHost := range virtualHosts {
		key := virtualHostDomainKey(virtualHost.Domains)
		index, ok := indexes[key]
		if ok {
			result[index].Routes = append(result[index].Routes, virtualHost.Routes...)
			continue
		}
		indexes[key] = len(result)
		result = append(result, targetxds.VirtualHost{
			Name:    virtualHost.Name,
			Domains: append([]string(nil), virtualHost.Domains...),
			Routes:  append([]targetxds.Route(nil), virtualHost.Routes...),
		})
	}
	return result
}

func virtualHostDomainKey(domains []string) string {
	sorted := append([]string(nil), domains...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x00")
}

func (b responseBuilder) buildRoutes(routes []targetxds.Route) ([]*routev3.Route, error) {
	resources := make([]*routev3.Route, 0)
	for _, route := range routes {
		methods := route.Match.Methods
		if len(methods) == 0 {
			methods = []string{""}
		}
		for _, method := range methods {
			resource, err := b.buildRoute(route, method)
			if err != nil {
				return nil, err
			}
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

func (b responseBuilder) buildRoute(route targetxds.Route, method string) (*routev3.Route, error) {
	prefix := route.Match.PathPrefix
	if prefix == "" {
		prefix = defaultRoutePrefix
	}

	headers := make([]*routev3.HeaderMatcher, 0, len(route.Match.Headers)+1)
	if method != "" {
		headers = append(headers, &routev3.HeaderMatcher{
			Name: ":method",
			HeaderMatchSpecifier: &routev3.HeaderMatcher_ExactMatch{
				ExactMatch: method,
			},
		})
	}
	for _, header := range route.Match.Headers {
		headers = append(headers, &routev3.HeaderMatcher{
			Name: header.Name,
			HeaderMatchSpecifier: &routev3.HeaderMatcher_ExactMatch{
				ExactMatch: header.Value,
			},
		})
	}

	clusters := make([]*routev3.WeightedCluster_ClusterWeight, 0, len(route.WeightedClusters))
	for _, cluster := range route.WeightedClusters {
		clusters = append(clusters, &routev3.WeightedCluster_ClusterWeight{
			Name:   cluster.Name,
			Weight: &wrapperspb.UInt32Value{Value: uint32(cluster.Weight)},
		})
	}
	if len(clusters) == 0 {
		return nil, fmt.Errorf("route %q has no weighted clusters", route.Name)
	}

	action := &routev3.RouteAction{
		ClusterSpecifier: &routev3.RouteAction_WeightedClusters{
			WeightedClusters: &routev3.WeightedCluster{Clusters: clusters},
		},
	}
	if route.TimeoutMillis > 0 {
		action.Timeout = durationpb.New(time.Duration(route.TimeoutMillis) * time.Millisecond)
	}
	if route.RetryPolicy != nil && route.RetryPolicy.Attempts > 0 {
		action.RetryPolicy = &routev3.RetryPolicy{
			RetryOn:    retryOnUpstreamTransient,
			NumRetries: wrapperspb.UInt32(uint32(route.RetryPolicy.Attempts)),
		}
		if route.RetryPolicy.PerTryTimeoutMillis > 0 {
			action.RetryPolicy.PerTryTimeout = durationpb.New(time.Duration(route.RetryPolicy.PerTryTimeoutMillis) * time.Millisecond)
		}
	}

	routeMatch := &routev3.RouteMatch{
		PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: prefix},
		Headers:       headers,
	}
	if route.Match.Path != "" {
		// AI 接口通常是固定 API path，例如 /v1/chat/completions
		// exact path 避免被更宽的 prefix route 抢走
		routeMatch.PathSpecifier = &routev3.RouteMatch_Path{Path: route.Match.Path}
	}

	return &routev3.Route{
		Name:                   routeRuntimeName(route.GatewayName, route.Name, route.RuleName, method),
		Match:                  routeMatch,
		Action:                 &routev3.Route_Route{Route: action},
		RequestHeadersToAdd:    b.requestHeadersToAdd(route.RequestHeadersToAdd),
		RequestHeadersToRemove: route.RequestHeadersToRemove,
	}, nil
}

func routeRuntimeName(gatewayName, routeName, ruleName, method string) string {
	return fmt.Sprintf(
		"%s/%s/%s/%s/%s",
		routeNamePrefix,
		url.PathEscape(gatewayName),
		url.PathEscape(routeName),
		url.PathEscape(ruleName),
		url.PathEscape(method),
	)
}

func (b responseBuilder) requestHeadersToAdd(headers []targetxds.HeaderValue) []*corev3.HeaderValueOption {
	result := make([]*corev3.HeaderValueOption, 0, len(headers))
	for _, header := range headers {
		result = append(result, &corev3.HeaderValueOption{
			Header: &corev3.HeaderValue{
				Key:   header.Name,
				Value: header.Value,
			},
			AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
		})
	}
	return result
}
