package compiler

import (
	"cmp"
	"fmt"
	"slices"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	tlsinspectorv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/tls_inspector/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"google.golang.org/protobuf/types/known/anypb"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

const (
	tlsInspectorFilterName = "envoy.filters.listener.tls_inspector"
	tlsTransportSocketName = "envoy.transport_sockets.tls"
	tlsTransportProtocol   = "tls"
)

func (c *compilation) configureHTTPSListener(
	listener *listenerv3.Listener,
	group *listenerGroup,
	hcm *anypb.Any,
) error {
	inspector, err := anypb.New(&tlsinspectorv3.TlsInspector{})
	if err != nil {
		return fmt.Errorf("encode TLS inspector for listener %s: %w", listener.Name, err)
	}
	listener.ListenerFilters = []*listenerv3.ListenerFilter{{
		Name:       tlsInspectorFilterName,
		ConfigType: &listenerv3.ListenerFilter_TypedConfig{TypedConfig: inspector},
	}}

	gatewayListeners := slices.Clone(group.gatewayListeners)
	slices.SortFunc(gatewayListeners, func(a, b gatewayListener) int {
		if result := cmp.Compare(a.gatewayID, b.gatewayID); result != 0 {
			return result
		}
		return cmp.Compare(a.hostname, b.hostname)
	})
	for _, gatewayListener := range gatewayListeners {
		filterChain, err := buildHTTPSFilterChain(
			listener.Name,
			gatewayListener,
			c.certificates[gatewayListener.certificateRef],
			hcm,
		)
		if err != nil {
			return err
		}
		if gatewayListener.hostname == "*" {
			listener.DefaultFilterChain = filterChain
		} else {
			listener.FilterChains = append(listener.FilterChains, filterChain)
		}
	}
	return nil
}

func buildHTTPSFilterChain(
	listenerName string,
	gatewayListener gatewayListener,
	certificate *gatewayv1.Certificate,
	hcm *anypb.Any,
) (*listenerv3.FilterChain, error) {
	tlsContext := &tlsv3.DownstreamTlsContext{CommonTlsContext: &tlsv3.CommonTlsContext{
		TlsCertificates: []*tlsv3.TlsCertificate{{
			CertificateChain: inlineStringDataSource(certificate.Spec.CertificatePEM),
			PrivateKey:       inlineStringDataSource(certificate.Spec.PrivateKeyPEM),
		}},
		AlpnProtocols: []string{"h2", "http/1.1"},
	}}
	if err := tlsContext.ValidateAll(); err != nil {
		return nil, fmt.Errorf(
			"validate TLS context for gateway %q on listener %s: %w",
			gatewayListener.gatewayID,
			listenerName,
			err,
		)
	}
	typedTLSContext, err := anypb.New(tlsContext)
	if err != nil {
		return nil, fmt.Errorf(
			"encode TLS context for gateway %q on listener %s: %w",
			gatewayListener.gatewayID,
			listenerName,
			err,
		)
	}
	filterChain := httpFilterChain(hcm)
	filterChain.Name = listenerName + "/gateway/" + gatewayListener.gatewayID + "/" + gatewayListener.hostname
	filterChain.TransportSocket = &corev3.TransportSocket{
		Name:       tlsTransportSocketName,
		ConfigType: &corev3.TransportSocket_TypedConfig{TypedConfig: typedTLSContext},
	}
	if gatewayListener.hostname != "*" {
		filterChain.FilterChainMatch = &listenerv3.FilterChainMatch{
			ServerNames:       []string{gatewayListener.hostname},
			TransportProtocol: tlsTransportProtocol,
		}
	}
	return filterChain, nil
}

func inlineStringDataSource(value string) *corev3.DataSource {
	return &corev3.DataSource{Specifier: &corev3.DataSource_InlineString{InlineString: value}}
}
