package config

import (
	"cmp"
	"fmt"
	"maps"
	"net/netip"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/lgc202/ingate/internal/modelprovider"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	defaultUpstreamConnectTimeout = 5 * time.Second
	systemClusterPrefix           = "ingate-system-"
	systemCABundlePath            = "/etc/ssl/certs/ca-certificates.crt"
)

func (c *compileContext) buildUpstreams() ([]*clusterv3.Cluster, []*endpointv3.ClusterLoadAssignment) {
	// 普通 Upstream 直接使用资源 ID；模型 Upstream 使用连接配置指纹隔离新旧 CDS/EDS 资源
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
		if !c.validUpstreamProtocol(upstream) {
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

		endpoints, usesDNS := c.buildUpstreamEndpoints(upstream)
		clusterName := id
		if upstream.Spec.Type == gatewayv1.UpstreamTypeModel {
			apiKey, credentialValid := c.upstreamAPIKey(upstream)
			if !credentialValid {
				continue
			}
			clusterName = modelClusterName(upstream, lbPolicy, apiKey)
		}
		cluster := &clusterv3.Cluster{
			Name:           clusterName,
			ConnectTimeout: durationpb.New(defaultUpstreamConnectTimeout),
			LbPolicy:       lbPolicy,
		}
		transportSocket, validTLS := c.upstreamTransportSocket(upstream)
		if !validTLS {
			continue
		}
		cluster.TransportSocket = transportSocket
		c.upstreamClusters[id] = clusterName
		assignment := &endpointv3.ClusterLoadAssignment{
			ClusterName: clusterName,
			Endpoints: []*endpointv3.LocalityLbEndpoints{
				{LbEndpoints: endpoints},
			},
		}
		if usesDNS {
			// Envoy 只会在 DNS cluster 中解析 hostname，不能把 hostname 伪装成 EDS socket address
			cluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_STRICT_DNS}
			cluster.LoadAssignment = assignment
		} else {
			cluster.ClusterDiscoveryType = &clusterv3.Cluster_Type{Type: clusterv3.Cluster_EDS}
			cluster.EdsClusterConfig = &clusterv3.Cluster_EdsClusterConfig{
				EdsConfig:   adsConfigSource(),
				ServiceName: clusterName,
			}
			assignments = append(assignments, assignment)
		}
		clusters = append(clusters, cluster)
	}

	return clusters, assignments
}

func modelClusterName(upstream *gatewayv1.Upstream, lbPolicy clusterv3.Cluster_LbPolicy, apiKey string) string {
	fields := []string{
		"protocol", string(upstream.Spec.Protocol),
		"connectTimeout", defaultUpstreamConnectTimeout.String(),
		"loadBalancePolicy", lbPolicy.String(),
		"apiKey", apiKey,
	}
	if upstream.Spec.Model != nil {
		fields = append(fields,
			"provider", string(upstream.Spec.Model.Provider),
			"apiBasePath", upstream.Spec.Model.APIBasePath,
		)
	}
	if upstream.Spec.TLS == nil {
		fields = append(fields, "tls", "disabled")
	} else {
		fields = append(fields,
			"tls", "enabled",
			"serverName", normalizedTLSServerName(upstream.Spec.TLS.ServerName),
			"trustedCA", systemCABundlePath,
			"alpn", "http/1.1",
		)
	}

	endpoints := make([]gatewayv1.Endpoint, 0, len(upstream.Spec.Endpoints))
	for _, endpoint := range upstream.Spec.Endpoints {
		if endpoint.Enabled {
			endpoints = append(endpoints, endpoint)
		}
	}
	slices.SortFunc(endpoints, func(a, b gatewayv1.Endpoint) int {
		if a.Address != b.Address {
			return cmp.Compare(a.Address, b.Address)
		}
		if a.Port != b.Port {
			return cmp.Compare(a.Port, b.Port)
		}
		if a.Weight != b.Weight {
			return cmp.Compare(a.Weight, b.Weight)
		}
		return cmp.Compare(a.Name, b.Name)
	})
	for _, endpoint := range endpoints {
		fields = append(fields,
			"endpoint",
			endpoint.Name,
			endpoint.Address,
			strconv.Itoa(endpoint.Port),
			strconv.Itoa(endpoint.Weight),
		)
	}
	return upstream.Name + "/ai/" + configFingerprint(fields...)
}

func (c *compileContext) validUpstreamProtocol(upstream *gatewayv1.Upstream) bool {
	protocolValid := true
	if upstream.Spec.Type == "" {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("upstream %q must declare a type", upstream.Name),
		)
		protocolValid = false
	} else {
		switch upstream.Spec.Type {
		case gatewayv1.UpstreamTypeApplication, gatewayv1.UpstreamTypeModel, gatewayv1.UpstreamTypeAgent, gatewayv1.UpstreamTypeMCP:
		default:
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindUpstream,
				upstream.Name,
				ReasonUnsupported,
				fmt.Sprintf("upstream %q uses unsupported type %q", upstream.Name, upstream.Spec.Type),
			)
			protocolValid = false
		}
	}
	switch upstream.Spec.Protocol {
	case gatewayv1.UpstreamProtocolHTTP:
		if upstream.Spec.Type == gatewayv1.UpstreamTypeModel {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindUpstream,
				upstream.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("model upstream %q cannot use the HTTP protocol", upstream.Name),
			)
			protocolValid = false
		}
	case gatewayv1.UpstreamProtocolOpenAI, gatewayv1.UpstreamProtocolAnthropic, gatewayv1.UpstreamProtocolGemini:
		if upstream.Spec.Type != gatewayv1.UpstreamTypeModel {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindUpstream,
				upstream.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("upstream %q uses model protocol %q without model type", upstream.Name, upstream.Spec.Protocol),
			)
			protocolValid = false
		}
	default:
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonUnsupported,
			fmt.Sprintf("upstream %q uses unsupported protocol %q", upstream.Name, upstream.Spec.Protocol),
		)
		protocolValid = false
	}
	if upstream.Spec.Type == gatewayv1.UpstreamTypeModel {
		if !c.validModelUpstream(upstream) {
			protocolValid = false
		}
	} else if upstream.Spec.Model != nil {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("non-model upstream %q must not declare model configuration", upstream.Name),
		)
		protocolValid = false
	}
	if upstream.Spec.Authentication != nil && upstream.Spec.Type != gatewayv1.UpstreamTypeModel {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("non-model upstream %q must not declare authentication", upstream.Name),
		)
		protocolValid = false
	}
	if upstream.Spec.Authentication != nil && upstream.Spec.TLS == nil {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("upstream %q must use TLS when authentication is configured", upstream.Name),
		)
		protocolValid = false
	}
	if upstream.Spec.Authentication != nil &&
		(upstream.Spec.Authentication.APIKey == nil || !modelprovider.ValidAPIKey(upstream.Spec.Authentication.APIKey.Value)) {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("upstream %q API key is missing or invalid", upstream.Name),
		)
		protocolValid = false
	}
	return protocolValid
}

func (c *compileContext) validModelUpstream(upstream *gatewayv1.Upstream) bool {
	if upstream.Spec.Model == nil {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("model upstream %q must declare model configuration", upstream.Name),
		)
		return false
	}

	valid := true
	definition, providerValid := modelprovider.Lookup(modelprovider.ID(upstream.Spec.Model.Provider))
	if !providerValid {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonUnsupported,
			fmt.Sprintf("model upstream %q uses unsupported provider %q", upstream.Name, upstream.Spec.Model.Provider),
		)
		valid = false
	} else if gatewayv1.UpstreamProtocol(definition.Protocol) != upstream.Spec.Protocol {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("model upstream %q provider %q requires protocol %q", upstream.Name, upstream.Spec.Model.Provider, definition.Protocol),
		)
		valid = false
	}

	basePath := upstream.Spec.Model.APIBasePath
	if basePath == "" || !strings.HasPrefix(basePath, "/") || strings.ContainsAny(basePath, "?#") || path.Clean(basePath) != basePath || (basePath != "/" && strings.HasSuffix(basePath, "/")) {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("model upstream %q has invalid API base path %q", upstream.Name, basePath),
		)
		valid = false
	}

	if len(upstream.Spec.Model.Models) == 0 {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("model upstream %q must declare at least one model", upstream.Name),
		)
		return false
	}
	seenModels := make(map[string]bool, len(upstream.Spec.Model.Models))
	enabledModels := 0
	for _, model := range upstream.Spec.Model.Models {
		if model.Name == "" || strings.TrimSpace(model.Name) != model.Name {
			c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec, fmt.Sprintf("model upstream %q contains an invalid model name %q", upstream.Name, model.Name))
			valid = false
			continue
		}
		if seenModels[model.Name] {
			c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonConflict, fmt.Sprintf("model upstream %q declares model %q more than once", upstream.Name, model.Name))
			valid = false
			continue
		}
		seenModels[model.Name] = true
		if model.DisplayName == "" || strings.TrimSpace(model.DisplayName) != model.DisplayName {
			c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec, fmt.Sprintf("model upstream %q model %q has an invalid display name", upstream.Name, model.Name))
			valid = false
		}
		if model.Enabled {
			enabledModels++
		}
	}
	if enabledModels == 0 {
		c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, upstream.Name, ReasonInvalidSpec, fmt.Sprintf("model upstream %q must enable at least one model", upstream.Name))
		valid = false
	}
	return valid
}

func (c *compileContext) upstreamTransportSocket(upstream *gatewayv1.Upstream) (*corev3.TransportSocket, bool) {
	if upstream.Spec.TLS == nil {
		return nil, true
	}

	serverName := normalizedTLSServerName(upstream.Spec.TLS.ServerName)
	if !validEndpointAddress(serverName) {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("upstream %q has invalid TLS server name %q", upstream.Name, serverName),
		)
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
					// Envoy 1.36 尚未实现 system_root_certs，显式读取数据面镜像中的系统 CA 根证书包
					TrustedCa: &corev3.DataSource{
						Specifier: &corev3.DataSource_Filename{Filename: systemCABundlePath},
					},
					MatchTypedSubjectAltNames: []*tlsv3.SubjectAltNameMatcher{
						{
							SanType: sanType,
							Matcher: &matcherv3.StringMatcher{
								MatchPattern: &matcherv3.StringMatcher_Exact{Exact: serverName},
							},
						},
					},
				},
			},
		},
	}
	if sanType == tlsv3.SubjectAltNameMatcher_DNS {
		tlsContext.Sni = serverName
	}
	if err := tlsContext.ValidateAll(); err != nil {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonCompileFailed,
			fmt.Sprintf("validate TLS context for upstream %q: %v", upstream.Name, err),
		)
		return nil, false
	}
	typedTLSContext, err := anypb.New(tlsContext)
	if err != nil {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonCompileFailed,
			fmt.Sprintf("encode TLS context for upstream %q: %v", upstream.Name, err),
		)
		return nil, false
	}
	return &corev3.TransportSocket{
		Name: tlsTransportSocketName,
		ConfigType: &corev3.TransportSocket_TypedConfig{
			TypedConfig: typedTLSContext,
		},
	}, true
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

func (c *compileContext) buildUpstreamEndpoints(upstream *gatewayv1.Upstream) ([]*endpointv3.LbEndpoint, bool) {
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
	usesDNS := false
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
				fmt.Sprintf("upstream %q endpoint %q address %q must be an IP address or DNS hostname", upstream.Name, endpoint.Name, endpoint.Address),
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
		usesDNS = usesDNS || !isIPAddress(endpoint.Address)

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
				Address: address,
				PortSpecifier: &corev3.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		},
	}
}
