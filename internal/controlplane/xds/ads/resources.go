package ads

import (
	"fmt"
	"strings"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	localratelimitv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/local_ratelimit/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"

	xdscache "github.com/lgc202/ingate/internal/controlplane/xds/cache"
	"github.com/lgc202/ingate/internal/controlplane/xds/translate"
)

func TypeURLForAlias(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	switch trimmed {
	case resource.ListenerType, resource.RouteType, resource.ClusterType, resource.EndpointType:
		return trimmed, nil
	}

	switch strings.ToLower(trimmed) {
	case "lds":
		return resource.ListenerType, nil
	case "rds":
		return resource.RouteType, nil
	case "cds":
		return resource.ClusterType, nil
	case "eds":
		return resource.EndpointType, nil
	default:
		return "", fmt.Errorf("unsupported xds resource type %q", value)
	}
}

func ResourceNameFromAny(resource *anypb.Any) (string, error) {
	if resource == nil {
		return "", fmt.Errorf("resource must not be nil")
	}

	message, err := resource.UnmarshalNew()
	if err != nil {
		return "", err
	}

	switch typed := message.(type) {
	case *listenerv3.Listener:
		return typed.GetName(), nil
	case *routev3.RouteConfiguration:
		return typed.GetName(), nil
	case *clusterv3.Cluster:
		return typed.GetName(), nil
	case *endpointv3.ClusterLoadAssignment:
		return typed.GetClusterName(), nil
	default:
		return "", fmt.Errorf("unsupported xds resource message %T", typed)
	}
}

func BuildResources(snapshot xdscache.Snapshot, typeURL string, resourceNames []string) ([]*anypb.Any, error) {
	switch typeURL {
	case resource.ListenerType:
		return marshalResources(filterMessages(resourceNames, buildListeners(snapshot)))
	case resource.RouteType:
		return marshalResources(filterMessages(resourceNames, buildRoutes(snapshot)))
	case resource.ClusterType:
		return marshalResources(filterMessages(resourceNames, buildClusters(snapshot)))
	case resource.EndpointType:
		return marshalResources(filterMessages(resourceNames, buildEndpoints(snapshot)))
	default:
		return nil, fmt.Errorf("unsupported type url %q", typeURL)
	}
}

func RouteConfigName(snapshot xdscache.Snapshot) string {
	return snapshot.Key.String() + "/routes"
}

func buildListeners(snapshot xdscache.Snapshot) []proto.Message {
	if snapshot.Runtime == nil {
		return nil
	}

	out := make([]proto.Message, 0, len(snapshot.Runtime.Listeners))
	for _, listener := range snapshot.Runtime.Listeners {
		routerConfig, _ := anypb.New(&routerv3.Router{})
		httpFilters := make([]*hcmv3.HttpFilter, 0, 2)
		if needsLocalRateLimitFilter(snapshot) {
			localRateLimitConfig, _ := anypb.New(&localratelimitv3.LocalRateLimit{
				StatPrefix:     nonEmpty(listener.Name, "ingate-http") + "-local-ratelimit",
				FilterEnabled:  disabledRuntimeFraction(),
				FilterEnforced: disabledRuntimeFraction(),
			})
			httpFilters = append(httpFilters, &hcmv3.HttpFilter{
				Name: "envoy.filters.http.local_ratelimit",
				ConfigType: &hcmv3.HttpFilter_TypedConfig{
					TypedConfig: localRateLimitConfig,
				},
			})
		}
		httpFilters = append(httpFilters, &hcmv3.HttpFilter{
			Name: "envoy.filters.http.router",
			ConfigType: &hcmv3.HttpFilter_TypedConfig{
				TypedConfig: routerConfig,
			},
		})
		manager, _ := anypb.New(&hcmv3.HttpConnectionManager{
			CodecType:  hcmv3.HttpConnectionManager_AUTO,
			StatPrefix: nonEmpty(listener.Name, "ingate-http"),
			RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
				Rds: &hcmv3.Rds{
					RouteConfigName: RouteConfigName(snapshot),
					ConfigSource:    adsConfigSource(),
				},
			},
			HttpFilters: httpFilters,
		})

		out = append(out, &listenerv3.Listener{
			Name: nonEmpty(listener.Name, "listener"),
			Address: &corev3.Address{
				Address: &corev3.Address_SocketAddress{
					SocketAddress: &corev3.SocketAddress{
						Address: "0.0.0.0",
						PortSpecifier: &corev3.SocketAddress_PortValue{
							PortValue: listener.Port,
						},
					},
				},
			},
			FilterChains: []*listenerv3.FilterChain{
				{
					Filters: []*listenerv3.Filter{
						{
							Name: "envoy.filters.network.http_connection_manager",
							ConfigType: &listenerv3.Filter_TypedConfig{
								TypedConfig: manager,
							},
						},
					},
				},
			},
		})
	}
	return out
}

func buildRoutes(snapshot xdscache.Snapshot) []proto.Message {
	if snapshot.Runtime == nil {
		return nil
	}

	virtualHosts := make([]*routev3.VirtualHost, 0, len(snapshot.Runtime.Routes))
	for _, route := range snapshot.Runtime.Routes {
		routeEntries := make([]*routev3.Route, 0, len(route.Rules))
		for _, rule := range route.Rules {
			action := buildRouteAction(rule, route.TrafficSummary, snapshot.Runtime.GatewayTrafficSummary)
			if action == nil {
				continue
			}
			routeEntry := &routev3.Route{
				Match:  buildRouteMatch(rule),
				Action: &routev3.Route_Route{Route: action},
			}
			if rateLimitConfig := buildRouteRateLimitConfig(route.TrafficSummary, snapshot.Runtime.GatewayTrafficSummary); rateLimitConfig != nil {
				typedConfig, _ := anypb.New(rateLimitConfig)
				routeEntry.TypedPerFilterConfig = map[string]*anypb.Any{
					"envoy.filters.http.local_ratelimit": typedConfig,
				}
			}
			routeEntries = append(routeEntries, routeEntry)
		}
		if len(routeEntries) == 0 {
			continue
		}

		domains := append([]string(nil), route.Hostnames...)
		if len(domains) == 0 {
			domains = []string{"*"}
		}
		virtualHosts = append(virtualHosts, &routev3.VirtualHost{
			Name:    nonEmpty(route.Name, "route"),
			Domains: domains,
			Routes:  routeEntries,
		})
	}

	if len(virtualHosts) == 0 {
		return nil
	}

	return []proto.Message{
		&routev3.RouteConfiguration{
			Name:         RouteConfigName(snapshot),
			VirtualHosts: virtualHosts,
		},
	}
}

func buildClusters(snapshot xdscache.Snapshot) []proto.Message {
	if snapshot.Runtime == nil {
		return nil
	}

	out := make([]proto.Message, 0, len(snapshot.Runtime.Backends))
	for _, backend := range snapshot.Runtime.Backends {
		out = append(out, &clusterv3.Cluster{
			Name:           nonEmpty(backend.Name, "backend"),
			ConnectTimeout: durationpb.New(2 * time.Second),
			ClusterDiscoveryType: &clusterv3.Cluster_Type{
				Type: clusterv3.Cluster_EDS,
			},
			EdsClusterConfig: &clusterv3.Cluster_EdsClusterConfig{
				EdsConfig: adsConfigSource(),
			},
			LbPolicy: clusterv3.Cluster_ROUND_ROBIN,
		})
	}
	return out
}

func buildEndpoints(snapshot xdscache.Snapshot) []proto.Message {
	if snapshot.Runtime == nil {
		return nil
	}

	out := make([]proto.Message, 0, len(snapshot.Runtime.Backends))
	for _, backend := range snapshot.Runtime.Backends {
		lbEndpoints := make([]*endpointv3.LbEndpoint, 0, len(backend.Endpoints))
		for _, endpoint := range backend.Endpoints {
			healthStatus := corev3.HealthStatus_HEALTHY
			if !endpoint.Healthy {
				healthStatus = corev3.HealthStatus_UNHEALTHY
			}
			lbEndpoints = append(lbEndpoints, &endpointv3.LbEndpoint{
				HealthStatus: healthStatus,
				LoadBalancingWeight: &wrapperspb.UInt32Value{
					Value: endpoint.Weight,
				},
				HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
					Endpoint: &endpointv3.Endpoint{
						Address: &corev3.Address{
							Address: &corev3.Address_SocketAddress{
								SocketAddress: &corev3.SocketAddress{
									Address: endpoint.Address,
									PortSpecifier: &corev3.SocketAddress_PortValue{
										PortValue: endpoint.Port,
									},
								},
							},
						},
					},
				},
			})
		}
		out = append(out, &endpointv3.ClusterLoadAssignment{
			ClusterName: nonEmpty(backend.Name, "backend"),
			Endpoints: []*endpointv3.LocalityLbEndpoints{
				{
					LbEndpoints: lbEndpoints,
				},
			},
		})
	}
	return out
}

func buildRouteAction(rule translate.RuntimeRouteRule, summaries ...*translate.RuntimeTrafficSummary) *routev3.RouteAction {
	if len(rule.BackendRefs) == 0 {
		return nil
	}

	action := &routev3.RouteAction{}
	trafficPolicy := routeTrafficPolicyFromSummaries(summaries...)
	if rule.URLRewrite != nil && rule.URLRewrite.ReplacePrefixMatch != "" {
		action.PrefixRewrite = rule.URLRewrite.ReplacePrefixMatch
	}
	if trafficPolicy.timeout != nil {
		action.Timeout = trafficPolicy.timeout
	}
	if trafficPolicy.retry != nil {
		action.RetryPolicy = trafficPolicy.retry
	}

	if len(rule.BackendRefs) == 1 {
		action.ClusterSpecifier = &routev3.RouteAction_Cluster{
			Cluster: rule.BackendRefs[0].Name,
		}
		return action
	}

	weighted := &routev3.WeightedCluster{
		Clusters: make([]*routev3.WeightedCluster_ClusterWeight, 0, len(rule.BackendRefs)),
	}
	for _, backendRef := range rule.BackendRefs {
		weighted.Clusters = append(weighted.Clusters, &routev3.WeightedCluster_ClusterWeight{
			Name:   backendRef.Name,
			Weight: &wrapperspb.UInt32Value{Value: backendRef.Weight},
		})
	}
	action.ClusterSpecifier = &routev3.RouteAction_WeightedClusters{
		WeightedClusters: weighted,
	}
	return action
}

type routeTrafficPolicy struct {
	timeout   *durationpb.Duration
	retry     *routev3.RetryPolicy
	rateLimit *routeRateLimitPolicy
}

type routeRateLimitPolicy struct {
	requests int32
	unit     string
}

func routeTrafficPolicyFromSummaries(summaries ...*translate.RuntimeTrafficSummary) routeTrafficPolicy {
	var out routeTrafficPolicy

	for _, summary := range summaries {
		if summary == nil {
			continue
		}
		for _, policy := range summary.Policies {
			if out.timeout == nil {
				if timeout := parseTimeoutDuration(policy.TimeoutDuration); timeout != nil {
					out.timeout = timeout
				}
			}
			if out.retry == nil && policy.RetryAttempts > 0 {
				retryOn := normalizeRetryConditions(policy.RetryConditions)
				if retryOn == "" {
					retryOn = "5xx"
				}
				out.retry = &routev3.RetryPolicy{
					RetryOn:    retryOn,
					NumRetries: &wrapperspb.UInt32Value{Value: uint32(policy.RetryAttempts)},
				}
				if out.timeout != nil {
					out.retry.PerTryTimeout = out.timeout
				}
			}
			if out.rateLimit == nil && policy.RateLimitRequests > 0 {
				if unitDuration(policy.RateLimitUnit) > 0 {
					out.rateLimit = &routeRateLimitPolicy{
						requests: policy.RateLimitRequests,
						unit:     policy.RateLimitUnit,
					}
				}
			}
		}
	}

	return out
}

func parseTimeoutDuration(value string) *durationpb.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	timeout, err := time.ParseDuration(trimmed)
	if err != nil || timeout <= 0 {
		return nil
	}
	return durationpb.New(timeout)
}

func normalizeRetryConditions(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}

	out := make([]string, 0, len(conditions))
	for _, condition := range conditions {
		trimmed := strings.ToLower(strings.TrimSpace(condition))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return strings.Join(out, ",")
}

func buildRouteRateLimitConfig(summaries ...*translate.RuntimeTrafficSummary) *localratelimitv3.LocalRateLimit {
	trafficPolicy := routeTrafficPolicyFromSummaries(summaries...)
	if trafficPolicy.rateLimit == nil {
		return nil
	}

	fillInterval := unitDuration(trafficPolicy.rateLimit.unit)
	if fillInterval <= 0 {
		return nil
	}

	return &localratelimitv3.LocalRateLimit{
		StatPrefix:     "ingate-local-ratelimit",
		FilterEnabled:  fullRuntimeFraction(),
		FilterEnforced: fullRuntimeFraction(),
		TokenBucket: &typev3.TokenBucket{
			MaxTokens: uint32(trafficPolicy.rateLimit.requests),
			TokensPerFill: &wrapperspb.UInt32Value{
				Value: uint32(trafficPolicy.rateLimit.requests),
			},
			FillInterval: durationpb.New(fillInterval),
		},
	}
}

func needsLocalRateLimitFilter(snapshot xdscache.Snapshot) bool {
	if snapshot.Runtime == nil {
		return false
	}
	if buildRouteRateLimitConfig(snapshot.Runtime.GatewayTrafficSummary) != nil {
		return true
	}
	for _, route := range snapshot.Runtime.Routes {
		if buildRouteRateLimitConfig(route.TrafficSummary, snapshot.Runtime.GatewayTrafficSummary) != nil {
			return true
		}
	}
	return false
}

func fullRuntimeFraction() *corev3.RuntimeFractionalPercent {
	return &corev3.RuntimeFractionalPercent{
		DefaultValue: &typev3.FractionalPercent{
			Numerator:   100,
			Denominator: typev3.FractionalPercent_HUNDRED,
		},
	}
}

func disabledRuntimeFraction() *corev3.RuntimeFractionalPercent {
	return &corev3.RuntimeFractionalPercent{
		DefaultValue: &typev3.FractionalPercent{
			Numerator:   0,
			Denominator: typev3.FractionalPercent_HUNDRED,
		},
	}
}

func unitDuration(unit string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "second":
		return time.Second
	case "minute":
		return time.Minute
	case "hour":
		return time.Hour
	default:
		return 0
	}
}

func buildRouteMatch(rule translate.RuntimeRouteRule) *routev3.RouteMatch {
	match := &routev3.RouteMatch{
		PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: "/"},
	}
	if len(rule.Matches) == 0 {
		return match
	}

	first := rule.Matches[0]
	if first.Path != nil {
		switch first.Path.Type {
		case "Exact":
			match.PathSpecifier = &routev3.RouteMatch_Path{Path: first.Path.Value}
		default:
			match.PathSpecifier = &routev3.RouteMatch_Prefix{Prefix: nonEmpty(first.Path.Value, "/")}
		}
	}
	if first.Method != "" {
		match.Headers = append(match.Headers, &routev3.HeaderMatcher{
			Name: ":method",
			HeaderMatchSpecifier: &routev3.HeaderMatcher_StringMatch{
				StringMatch: &matcherv3.StringMatcher{
					MatchPattern: &matcherv3.StringMatcher_Exact{Exact: first.Method},
				},
			},
		})
	}
	for _, header := range first.Headers {
		match.Headers = append(match.Headers, &routev3.HeaderMatcher{
			Name: header.Name,
			HeaderMatchSpecifier: &routev3.HeaderMatcher_StringMatch{
				StringMatch: &matcherv3.StringMatcher{
					MatchPattern: &matcherv3.StringMatcher_Exact{Exact: header.Value},
				},
			},
		})
	}
	return match
}

func marshalResources(messages []proto.Message) ([]*anypb.Any, error) {
	out := make([]*anypb.Any, 0, len(messages))
	for _, message := range messages {
		item, err := anypb.New(message)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func filterMessages(resourceNames []string, messages []proto.Message) []proto.Message {
	if len(resourceNames) == 0 {
		return messages
	}

	allowed := make(map[string]struct{}, len(resourceNames))
	for _, name := range resourceNames {
		if strings.TrimSpace(name) == "" {
			continue
		}
		allowed[name] = struct{}{}
	}
	if len(allowed) == 0 {
		return messages
	}

	filtered := make([]proto.Message, 0, len(messages))
	for _, message := range messages {
		name := resourceMessageName(message)
		if _, ok := allowed[name]; !ok {
			continue
		}
		filtered = append(filtered, message)
	}
	return filtered
}

func resourceMessageName(message proto.Message) string {
	switch typed := message.(type) {
	case *listenerv3.Listener:
		return typed.GetName()
	case *routev3.RouteConfiguration:
		return typed.GetName()
	case *clusterv3.Cluster:
		return typed.GetName()
	case *endpointv3.ClusterLoadAssignment:
		return typed.GetClusterName()
	default:
		return ""
	}
}

func adsConfigSource() *corev3.ConfigSource {
	return &corev3.ConfigSource{
		ResourceApiVersion: corev3.ApiVersion_V3,
		ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
			Ads: &corev3.AggregatedConfigSource{},
		},
	}
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
