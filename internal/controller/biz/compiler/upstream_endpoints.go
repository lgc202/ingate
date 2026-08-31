package compiler

import (
	"cmp"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
)

func (c *compilation) buildUpstreamEndpoints(
	upstream *gatewayv1.Upstream,
	modelProtocol aiprotocol.UpstreamProtocol,
) ([]*endpointv3.LbEndpoint, bool, bool) {
	endpointCount := len(upstream.Spec.Endpoints)
	if endpointCount == 0 {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("upstream %q must declare at least one endpoint", upstream.Name),
		)
		return nil, false, false
	}
	if endpointCount > upstreamconfig.MaxEndpoints {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"upstream %q declares %d endpoints; the maximum is %d",
				upstream.Name,
				endpointCount,
				upstreamconfig.MaxEndpoints,
			),
		)
		return nil, false, false
	}

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
	seenEndpointKeys := make(map[string]bool, len(items))
	usesDNS := false
	valid := true
	for _, endpoint := range items {
		endpointKey := net.JoinHostPort(endpoint.Address, strconv.Itoa(endpoint.Port))
		if seenEndpointKeys[endpointKey] {
			c.addResourceError(
				gatewayv1.KindUpstream,
				upstream.Name,
				ReasonInvalidSpec,
				fmt.Sprintf(
					"upstream %q declares endpoint %q more than once",
					upstream.Name,
					endpointKey,
				),
			)
			valid = false
			continue
		}
		seenEndpointKeys[endpointKey] = true
		if !c.validUpstreamEndpoint(upstream, endpoint, endpointKey) {
			valid = false
			continue
		}
		usesDNS = usesDNS || !isIPAddress(endpoint.Address)
		// AutoHostRewrite 依赖 Endpoint.Hostname；IP 端点也必须显式提供该值，
		// 否则 UpstreamHost 模式会在 EDS Cluster 中静默保留客户端 Host。
		envoyEndpoint := &endpointv3.Endpoint{
			Address:  socketAddress(endpoint.Address, endpoint.Port),
			Hostname: strings.ToLower(endpoint.Address),
		}
		lbEndpoint := &endpointv3.LbEndpoint{
			HostIdentifier:      &endpointv3.LbEndpoint_Endpoint{Endpoint: envoyEndpoint},
			LoadBalancingWeight: wrapperspb.UInt32(uint32(endpoint.Weight)),
		}
		if modelProtocol != "" {
			// 端点元数据在负载均衡完成后才可用，
			// 用于告诉上游 ExtProc 本次实际选中的模型 Service。
			lbEndpoint.Metadata = &corev3.Metadata{FilterMetadata: map[string]*structpb.Struct{
				aiprotocol.MetadataNamespace: {
					Fields: map[string]*structpb.Value{
						aiprotocol.ServiceIDField:       structpb.NewStringValue(upstream.Name),
						aiprotocol.ServiceProtocolField: structpb.NewStringValue(string(modelProtocol)),
					},
				},
			}}
		}
		result = append(result, lbEndpoint)
	}
	return result, usesDNS, valid
}

func (c *compilation) validUpstreamEndpoint(
	upstream *gatewayv1.Upstream,
	endpoint gatewayv1.Endpoint,
	endpointKey string,
) bool {
	if !upstreamconfig.IsValidAddress(endpoint.Address) {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"upstream %q endpoint address %q must be an IP address or DNS hostname",
				upstream.Name,
				endpoint.Address,
			),
		)
		return false
	}
	if !upstreamconfig.IsValidEndpointPort(endpoint.Port) {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"upstream %q endpoint %q has invalid port %d",
				upstream.Name,
				endpointKey,
				endpoint.Port,
			),
		)
		return false
	}
	if !upstreamconfig.IsValidEndpointWeight(endpoint.Weight) {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"upstream %q endpoint %q has invalid weight %d",
				upstream.Name,
				endpointKey,
				endpoint.Weight,
			),
		)
		return false
	}
	return true
}

func isIPAddress(address string) bool {
	_, err := netip.ParseAddr(address)
	return err == nil
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
