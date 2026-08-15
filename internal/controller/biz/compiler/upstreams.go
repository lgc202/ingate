package compiler

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
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"k8s.io/apimachinery/pkg/util/validation"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultUpstreamConnectTimeout = 5 * time.Second
	hostnameDNSLookupFamily       = clusterv3.Cluster_V4_PREFERRED
	systemClusterPrefix           = "ingate-system-"
	systemCABundlePath            = "/etc/ssl/certs/ca-certificates.crt"
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
			c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, id, ReasonConflict, fmt.Sprintf("upstream %q uses reserved system cluster prefix %q", id, systemClusterPrefix))
			continue
		}

		lbPolicy, ok := c.upstreamLBPolicy(upstream)
		if !ok {
			continue
		}
		endpoints, usesDNS := c.buildUpstreamEndpoints(upstream)
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

func (c *compilation) upstreamTransportSocket(upstream *gatewayv1.Upstream) (*corev3.TransportSocket, bool) {
	if upstream.Spec.TLS == nil {
		return nil, true
	}

	serverName := normalizedTLSServerName(upstream.Spec.TLS.ServerName)
	if !validEndpointAddress(serverName) {
		c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec, fmt.Sprintf("upstream %q has invalid TLS server name %q", upstream.Name, serverName))
		return nil, false
	}

	sanType := tlsv3.SubjectAltNameMatcher_DNS
	if isIPAddress(serverName) {
		sanType = tlsv3.SubjectAltNameMatcher_IP_ADDRESS
	}
	tlsContext := &tlsv3.UpstreamTlsContext{
		CommonTlsContext: &tlsv3.CommonTlsContext{
			AlpnProtocols: []string{"http/1.1"},
			ValidationContextType: &tlsv3.CommonTlsContext_ValidationContext{
				ValidationContext: &tlsv3.CertificateValidationContext{
					// Envoy 读取镜像中的系统 CA，并同时校验证书身份
					TrustedCa: &corev3.DataSource{Specifier: &corev3.DataSource_Filename{Filename: systemCABundlePath}},
					MatchTypedSubjectAltNames: []*tlsv3.SubjectAltNameMatcher{{
						SanType: sanType,
						Matcher: &matcherv3.StringMatcher{MatchPattern: &matcherv3.StringMatcher_Exact{Exact: serverName}},
					}},
				},
			},
		},
	}
	if sanType == tlsv3.SubjectAltNameMatcher_DNS {
		tlsContext.Sni = serverName
	}
	if err := tlsContext.ValidateAll(); err != nil {
		c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonCompileFailed, fmt.Sprintf("validate TLS context for upstream %q: %v", upstream.Name, err))
		return nil, false
	}
	typedTLSContext, err := anypb.New(tlsContext)
	if err != nil {
		c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonCompileFailed, fmt.Sprintf("encode TLS context for upstream %q: %v", upstream.Name, err))
		return nil, false
	}
	return &corev3.TransportSocket{
		Name:       tlsTransportSocketName,
		ConfigType: &corev3.TransportSocket_TypedConfig{TypedConfig: typedTLSContext},
	}, true
}

func (c *compilation) upstreamLBPolicy(upstream *gatewayv1.Upstream) (clusterv3.Cluster_LbPolicy, bool) {
	switch upstream.Spec.LoadBalancing {
	case "", gatewayv1.LoadBalancingRoundRobin:
		return clusterv3.Cluster_ROUND_ROBIN, true
	case gatewayv1.LoadBalancingLeastRequest:
		return clusterv3.Cluster_LEAST_REQUEST, true
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonUnsupported, fmt.Sprintf("upstream %q uses unsupported load balancing policy %q", upstream.Name, upstream.Spec.LoadBalancing))
		return clusterv3.Cluster_ROUND_ROBIN, false
	}
}

func (c *compilation) buildUpstreamEndpoints(upstream *gatewayv1.Upstream) ([]*endpointv3.LbEndpoint, bool) {
	items := slices.Clone(upstream.Spec.Endpoints)
	slices.SortFunc(items, func(a, b gatewayv1.Endpoint) int {
		if a.Address != b.Address {
			return cmp.Compare(a.Address, b.Address)
		}
		if a.Port != b.Port {
			return cmp.Compare(a.Port, b.Port)
		}
		return cmp.Compare(a.Weight, b.Weight)
	})

	result := make([]*endpointv3.LbEndpoint, 0, len(items))
	seen := make(map[string]bool, len(items))
	usesDNS := false
	for _, endpoint := range items {
		key := fmt.Sprintf("%s:%d", endpoint.Address, endpoint.Port)
		if seen[key] {
			c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonConflict, fmt.Sprintf("upstream %q declares endpoint %q more than once", upstream.Name, key))
			continue
		}
		seen[key] = true
		if !validEndpointAddress(endpoint.Address) {
			c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec, fmt.Sprintf("upstream %q endpoint address %q must be an IP address or DNS hostname", upstream.Name, endpoint.Address))
			continue
		}
		if endpoint.Port < 1 || endpoint.Port > 65535 || endpoint.Weight < 1 || endpoint.Weight > 1000 {
			c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec, fmt.Sprintf("upstream %q endpoint %q has invalid port or weight", upstream.Name, key))
			continue
		}
		usesDNS = usesDNS || !isIPAddress(endpoint.Address)
		result = append(result, &endpointv3.LbEndpoint{
			HostIdentifier:      &endpointv3.LbEndpoint_Endpoint{Endpoint: &endpointv3.Endpoint{Address: socketAddress(endpoint.Address, endpoint.Port)}},
			LoadBalancingWeight: wrapperspb.UInt32(uint32(endpoint.Weight)),
		})
	}
	if len(items) == 0 {
		c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec, fmt.Sprintf("upstream %q must declare at least one endpoint", upstream.Name))
	}
	return result, usesDNS
}

func validEndpointAddress(address string) bool {
	if address == "" || strings.TrimSpace(address) != address {
		return false
	}
	if isIPAddress(address) {
		return true
	}
	return len(validation.IsDNS1123Subdomain(strings.ToLower(address))) == 0
}

func isIPAddress(address string) bool {
	_, err := netip.ParseAddr(address)
	return err == nil
}

func normalizedTLSServerName(serverName string) string {
	if isIPAddress(serverName) {
		return serverName
	}
	return strings.ToLower(serverName)
}

func socketAddress(address string, port int) *corev3.Address {
	return &corev3.Address{
		Address: &corev3.Address_SocketAddress{
			SocketAddress: &corev3.SocketAddress{
				Address:       address,
				PortSpecifier: &corev3.SocketAddress_PortValue{PortValue: uint32(port)},
			},
		},
	}
}
