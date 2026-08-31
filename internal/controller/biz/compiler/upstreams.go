package compiler

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
)

const (
	defaultUpstreamConnectTimeout = 5 * time.Second
	hostnameDNSLookupFamily       = clusterv3.Cluster_V4_PREFERRED
	systemClusterPrefix           = "ingate-system-"
)

func (c *compilation) buildUpstreams() (
	[]*clusterv3.Cluster,
	[]*endpointv3.ClusterLoadAssignment,
	map[string]bool,
) {
	ids := slices.Sorted(maps.Keys(c.upstreams))
	clusters := make([]*clusterv3.Cluster, 0, len(ids))
	assignments := make([]*endpointv3.ClusterLoadAssignment, 0, len(ids))
	compiledUpstreams := make(map[string]bool, len(ids))

	for _, id := range ids {
		upstream := c.upstreams[id]
		if strings.HasPrefix(id, systemClusterPrefix) {
			c.addResourceError(
				gatewayv1.KindUpstream,
				id,
				ReasonConflict,
				fmt.Sprintf(
					"upstream %q uses reserved system cluster prefix %q",
					id,
					systemClusterPrefix,
				),
			)
			continue
		}

		lbPolicy, ok := c.upstreamLBPolicy(upstream)
		if !ok {
			continue
		}
		modelProtocol, ok := c.upstreamModelProtocol(upstream)
		if !ok {
			continue
		}
		healthCheckValid := c.validUpstreamHealthCheck(upstream)
		endpoints, usesDNS, endpointsValid := c.buildUpstreamEndpoints(upstream, modelProtocol)
		if !healthCheckValid || !endpointsValid {
			continue
		}
		cluster := &clusterv3.Cluster{
			Name:           id,
			ConnectTimeout: durationpb.New(defaultUpstreamConnectTimeout),
			LbPolicy:       lbPolicy,
		}
		transportSocket, validTLS := c.upstreamTransportSocket(upstream)
		if !validTLS {
			continue
		}
		cluster.TransportSocket = transportSocket
		if upstream.Spec.HealthCheck != nil {
			cluster.HealthChecks = []*corev3.HealthCheck{buildHealthCheck(upstream.Spec.HealthCheck)}
		}
		if modelProtocol != "" {
			protocolOptions, err := buildAIUpstreamProtocolOptions()
			if err != nil {
				c.addResourceError(
					gatewayv1.KindUpstream,
					id,
					ReasonCompileFailed,
					fmt.Sprintf("compile model upstream %q: %v", id, err),
				)
				continue
			}
			cluster.TypedExtensionProtocolOptions = map[string]*anypb.Any{
				httpProtocolOptionsName: protocolOptions,
			}
		}

		assignment := &endpointv3.ClusterLoadAssignment{
			ClusterName: id,
			Endpoints:   []*endpointv3.LocalityLbEndpoints{{LbEndpoints: endpoints}},
		}
		if usesDNS {
			cluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_STRICT_DNS}
			cluster.DnsLookupFamily = hostnameDNSLookupFamily
			cluster.LoadAssignment = assignment
		} else {
			cluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_EDS}
			cluster.EdsClusterConfig = &clusterv3.Cluster_EdsClusterConfig{
				EdsConfig:   adsConfigSource(),
				ServiceName: id,
			}
			assignments = append(assignments, assignment)
		}
		compiledUpstreams[id] = true
		clusters = append(clusters, cluster)
	}
	return clusters, assignments, compiledUpstreams
}

func buildHealthCheck(config *gatewayv1.UpstreamHealthCheck) *corev3.HealthCheck {
	return &corev3.HealthCheck{
		Timeout:            durationpb.New(time.Duration(config.TimeoutSeconds) * time.Second),
		Interval:           durationpb.New(time.Duration(config.IntervalSeconds) * time.Second),
		UnhealthyThreshold: wrapperspb.UInt32(3),
		HealthyThreshold:   wrapperspb.UInt32(2),
		HealthChecker: &corev3.HealthCheck_HttpHealthCheck_{
			HttpHealthCheck: &corev3.HealthCheck_HttpHealthCheck{Path: config.Path},
		},
	}
}

func (c *compilation) upstreamLBPolicy(upstream *gatewayv1.Upstream) (clusterv3.Cluster_LbPolicy, bool) {
	switch upstream.Spec.LoadBalancing {
	case "", gatewayv1.LoadBalancingRoundRobin:
		return clusterv3.Cluster_ROUND_ROBIN, true
	case gatewayv1.LoadBalancingLeastRequest:
		return clusterv3.Cluster_LEAST_REQUEST, true
	default:
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonUnsupported,
			fmt.Sprintf(
				"upstream %q uses unsupported load balancing policy %q",
				upstream.Name,
				upstream.Spec.LoadBalancing,
			),
		)
		return clusterv3.Cluster_ROUND_ROBIN, false
	}
}

func (c *compilation) upstreamModelProtocol(upstream *gatewayv1.Upstream) (aiprotocol.UpstreamProtocol, bool) {
	if upstream.Spec.Model == nil {
		return "", true
	}
	if !upstreamconfig.IsValidModelAPIKey(upstream.Spec.Model.APIKey) {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("upstream %q contains an invalid model API key", upstream.Name),
		)
		return "", false
	}

	switch upstream.Spec.Model.Protocol {
	case gatewayv1.ModelProtocolOpenAI:
		return aiprotocol.UpstreamProtocolOpenAI, true
	case gatewayv1.ModelProtocolAnthropic:
		return aiprotocol.UpstreamProtocolAnthropic, true
	default:
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonUnsupported,
			fmt.Sprintf(
				"upstream %q uses unsupported model protocol %q",
				upstream.Name,
				upstream.Spec.Model.Protocol,
			),
		)
		return "", false
	}
}

func (c *compilation) validUpstreamHealthCheck(upstream *gatewayv1.Upstream) bool {
	healthCheck := upstream.Spec.HealthCheck
	if healthCheck == nil {
		return true
	}
	if !upstreamconfig.IsValidHealthCheckPath(healthCheck.Path) {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("upstream %q has invalid health check path %q", upstream.Name, healthCheck.Path),
		)
		return false
	}
	if !upstreamconfig.IsValidHealthCheckInterval(healthCheck.IntervalSeconds) {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"upstream %q has invalid health check interval %d seconds",
				upstream.Name,
				healthCheck.IntervalSeconds,
			),
		)
		return false
	}
	if !upstreamconfig.IsValidHealthCheckTimeout(
		healthCheck.TimeoutSeconds,
		healthCheck.IntervalSeconds,
	) {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"upstream %q has invalid health check timeout %d seconds",
				upstream.Name,
				healthCheck.TimeoutSeconds,
			),
		)
		return false
	}
	return true
}
