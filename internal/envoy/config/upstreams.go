package config

import (
	"cmp"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strings"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	defaultUpstreamConnectTimeout = 5 * time.Second
	systemClusterPrefix           = "ingate-system-"
)

func (c *compileContext) buildUpstreams() ([]*clusterv3.Cluster, []*endpointv3.ClusterLoadAssignment) {
	// Upstream ID 直接作为全局 Cluster identity，Route、CDS 和 EDS 始终使用同一个名字
	ids := slices.Sorted(maps.Keys(c.upstreams))
	clusters := make([]*clusterv3.Cluster, 0, len(ids))
	assignments := make([]*endpointv3.ClusterLoadAssignment, 0, len(ids))

	for _, id := range ids {
		upstream := c.upstreams[id]
		if strings.HasPrefix(id, systemClusterPrefix) {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindUpstream,
				id,
				ReasonConflict,
				fmt.Sprintf("upstream %q uses reserved system cluster prefix %q", id, systemClusterPrefix),
			)
			continue
		}

		lbPolicy, ok := c.upstreamLBPolicy(upstream)
		if !ok {
			continue
		}
		if upstream.Spec.HealthCheck != nil && upstream.Spec.HealthCheck.Enabled {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindUpstream,
				id,
				ReasonUnsupported,
				fmt.Sprintf("upstream %q enables unsupported active health checks", id),
			)
		}

		endpoints := c.buildUpstreamEndpoints(upstream)
		clusters = append(clusters, &clusterv3.Cluster{
			Name:           id,
			ConnectTimeout: durationpb.New(defaultUpstreamConnectTimeout),
			LbPolicy:       lbPolicy,
			ClusterDiscoveryType: &clusterv3.Cluster_Type{
				Type: clusterv3.Cluster_EDS,
			},
			EdsClusterConfig: &clusterv3.Cluster_EdsClusterConfig{
				EdsConfig:   adsConfigSource(),
				ServiceName: id,
			},
		})
		assignments = append(assignments, &endpointv3.ClusterLoadAssignment{
			ClusterName: id,
			Endpoints: []*endpointv3.LocalityLbEndpoints{
				{LbEndpoints: endpoints},
			},
		})
	}

	return clusters, assignments
}

func (c *compileContext) upstreamLBPolicy(upstream *gatewayv1.Upstream) (clusterv3.Cluster_LbPolicy, bool) {
	switch upstream.Spec.LoadBalancePolicy {
	case "", gatewayv1.UpstreamLoadBalancePolicyRoundRobin:
		return clusterv3.Cluster_ROUND_ROBIN, true
	case gatewayv1.UpstreamLoadBalancePolicyLeastRequest:
		return clusterv3.Cluster_LEAST_REQUEST, true
	case gatewayv1.UpstreamLoadBalancePolicyRandom:
		return clusterv3.Cluster_RANDOM, true
	default:
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonUnsupported,
			fmt.Sprintf("upstream %q uses unsupported load balance policy %q", upstream.Name, upstream.Spec.LoadBalancePolicy),
		)
		return clusterv3.Cluster_ROUND_ROBIN, false
	}
}

func (c *compileContext) buildUpstreamEndpoints(upstream *gatewayv1.Upstream) []*endpointv3.LbEndpoint {
	items := slices.Clone(upstream.Spec.Endpoints)
	slices.SortFunc(items, func(a, b gatewayv1.Endpoint) int {
		if a.Address != b.Address {
			return cmp.Compare(a.Address, b.Address)
		}
		if a.Port != b.Port {
			return cmp.Compare(a.Port, b.Port)
		}
		return cmp.Compare(a.Name, b.Name)
	})

	result := make([]*endpointv3.LbEndpoint, 0, len(items))
	seenNames := make(map[string]bool, len(items))
	enabledCount := 0
	for _, endpoint := range items {
		valid := true
		if endpoint.Name != "" && seenNames[endpoint.Name] {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindUpstream,
				upstream.Name,
				ReasonConflict,
				fmt.Sprintf("upstream %q has duplicate endpoint %q", upstream.Name, endpoint.Name),
			)
			valid = false
		}
		seenNames[endpoint.Name] = endpoint.Name != ""

		if !validEndpointAddress(endpoint.Address) {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindUpstream,
				upstream.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("upstream %q endpoint %q has invalid address %q", upstream.Name, endpoint.Name, endpoint.Address),
			)
			valid = false
		}
		if endpoint.Port < 1 || endpoint.Port > 65535 {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindUpstream,
				upstream.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("upstream %q endpoint %q port must be between 1 and 65535", upstream.Name, endpoint.Name),
			)
			valid = false
		}
		if endpoint.Weight < 1 || endpoint.Weight > 100 {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindUpstream,
				upstream.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("upstream %q endpoint %q weight must be between 1 and 100", upstream.Name, endpoint.Name),
			)
			valid = false
		}
		if !endpoint.Enabled {
			continue
		}
		enabledCount++
		if !valid {
			continue
		}

		result = append(result, &endpointv3.LbEndpoint{
			HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
				Endpoint: &endpointv3.Endpoint{
					Address: socketAddress(endpoint.Address, endpoint.Port),
				},
			},
			LoadBalancingWeight: wrapperspb.UInt32(uint32(endpoint.Weight)),
		})
	}

	if len(items) == 0 {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("upstream %q must declare at least one endpoint", upstream.Name),
		)
	} else if enabledCount == 0 {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("upstream %q must have at least one enabled endpoint", upstream.Name),
		)
	}

	return result
}

func validEndpointAddress(address string) bool {
	if address == "" || strings.TrimSpace(address) != address {
		return false
	}
	if _, err := netip.ParseAddr(address); err == nil {
		return true
	}
	return validDNSName(strings.ToLower(address))
}

func socketAddress(address string, port int) *corev3.Address {
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
