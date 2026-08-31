package compiler

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/types/known/anypb"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
)

const systemCABundlePath = "/etc/ssl/certs/ca-certificates.crt"

func (c *compilation) upstreamTransportSocket(
	upstream *gatewayv1.Upstream,
) (*corev3.TransportSocket, bool) {
	if upstream.Spec.TLS == nil {
		return nil, true
	}

	serverName := upstreamconfig.NormalizeAddress(upstream.Spec.TLS.ServerName)
	if !upstreamconfig.IsValidAddress(serverName) {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"upstream %q has invalid TLS server name %q",
				upstream.Name,
				serverName,
			),
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
					// Envoy 读取镜像中的系统 CA，并同时校验证书身份。
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
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonCompileFailed,
			fmt.Sprintf("validate TLS context for upstream %q: %v", upstream.Name, err),
		)
		return nil, false
	}
	typedTLSContext, err := anypb.New(tlsContext)
	if err != nil {
		c.addResourceError(
			gatewayv1.KindUpstream,
			upstream.Name,
			ReasonCompileFailed,
			fmt.Sprintf("encode TLS context for upstream %q: %v", upstream.Name, err),
		)
		return nil, false
	}
	return &corev3.TransportSocket{
		Name:       tlsTransportSocketName,
		ConfigType: &corev3.TransportSocket_TypedConfig{TypedConfig: typedTLSContext},
	}, true
}
