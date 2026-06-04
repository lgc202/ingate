package server

import (
	"fmt"
	"sort"
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
)

func (b responseBuilder) buildRouteConfigs(configs []snapshotConfig) ([]*anypb.Any, error) {
	resources := make([]*anypb.Any, 0)
	groups := map[listenerGroupKey][]targetxds.VirtualHost{}
	unlinked := map[string][]targetxds.VirtualHost{}

	for _, config := range configs {
		listenerKeys := routeConfigListenerKeys(config.Config)
		for _, routeConfig := range config.Config.RouteConfigs {
			if key, ok := listenerKeys[routeConfig.Name]; ok {
				groups[key] = append(groups[key], routeConfig.VirtualHosts...)
				continue
			}
			unlinked[routeConfig.Name] = append(unlinked[routeConfig.Name], routeConfig.VirtualHosts...)
		}
	}

	keys := map[listenerGroupKey]struct{}{}
	for key := range groups {
		keys[key] = struct{}{}
	}
	for _, key := range sortedListenerKeys(keys) {
		resource, err := b.buildRouteConfig(listenerRouteConfigName(key), groups[key])
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
		resource, err := b.buildRouteConfig(name, unlinked[name])
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (b responseBuilder) buildRouteConfig(name string, virtualHosts []targetxds.VirtualHost) (*anypb.Any, error) {
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

	name := route.Name
	if method != "" {
		name = fmt.Sprintf("%s/%s", route.Name, method)
	}
	return &routev3.Route{
		Name:                   name,
		Match:                  routeMatch,
		Action:                 &routev3.Route_Route{Route: action},
		RequestHeadersToAdd:    b.requestHeadersToAdd(route.RequestHeadersToAdd),
		RequestHeadersToRemove: route.RequestHeadersToRemove,
	}, nil
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
