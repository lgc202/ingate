package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	tlsinspectorv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/listener/tls_inspector/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	defaultBindAddress              = "0.0.0.0"
	httpConnectionManagerFilterName = "envoy.filters.network.http_connection_manager"
	httpRouterFilterName            = "envoy.filters.http.router"
	tlsInspectorFilterName          = "envoy.filters.listener.tls_inspector"
	tlsTransportSocketName          = "envoy.transport_sockets.tls"
	tlsTransportProtocol            = "tls"
)

type listenerKey struct {
	address  string
	port     int
	protocol gatewayv1.Protocol
}

type listenerGroup struct {
	key      listenerKey
	claims   []hostnameClaim
	gateways []gatewayListener
}

type gatewayListener struct {
	key            listenerKey
	gatewayID      string
	hosts          []string
	certificateRef string
}

type hostnameClaim struct {
	gatewayID string
	hostname  string
}

func (c *compilation) buildListenerGroups() {
	for _, gatewayID := range slices.Sorted(maps.Keys(c.gateways)) {
		gateway := c.gateways[gatewayID]
		if !gateway.Spec.Enabled {
			continue
		}
		if len(gateway.Spec.Listeners) == 0 {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidSpec, fmt.Sprintf("gateway %q must declare at least one listener", gatewayID))
			continue
		}

		names := make(map[string]bool, len(gateway.Spec.Listeners))
		for _, listener := range gateway.Spec.Listeners {
			if listener.Name == "" || names[listener.Name] {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonConflict, fmt.Sprintf("gateway %q has an empty or duplicate listener name %q", gatewayID, listener.Name))
				continue
			}
			names[listener.Name] = true
			if listener.Port < 1 || listener.Port > 65535 {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidSpec, fmt.Sprintf("gateway %q listener %q has an invalid port", gatewayID, listener.Name))
				continue
			}
			if listener.Protocol != gatewayv1.ProtocolHTTP && listener.Protocol != gatewayv1.ProtocolHTTPS {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonUnsupported, fmt.Sprintf("gateway %q listener %q uses unsupported protocol %q", gatewayID, listener.Name, listener.Protocol))
				continue
			}

			hostname, ok := hostnameutil.Normalize(listener.Hostname)
			if !ok {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidSpec, fmt.Sprintf("gateway %q listener %q has invalid hostname %q", gatewayID, listener.Name, listener.Hostname))
				continue
			}
			if listener.Protocol == gatewayv1.ProtocolHTTP && listener.CertificateRef != "" {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidSpec, fmt.Sprintf("gateway %q HTTP listener %q must not reference a certificate", gatewayID, listener.Name))
				continue
			}
			if listener.Protocol == gatewayv1.ProtocolHTTPS && !c.validListenerCertificate(gatewayID, listener) {
				continue
			}

			key := listenerKey{address: defaultBindAddress, port: listener.Port, protocol: listener.Protocol}
			group := c.listenerGroups[key]
			if group == nil {
				group = &listenerGroup{key: key}
				c.listenerGroups[key] = group
			}
			gatewayListener := gatewayListener{
				key:            key,
				gatewayID:      gatewayID,
				hosts:          []string{hostname},
				certificateRef: listener.CertificateRef,
			}
			group.claims = append(group.claims, hostnameClaim{gatewayID: gatewayID, hostname: hostname})
			group.gateways = append(group.gateways, gatewayListener)
			c.gatewayListeners[gatewayID] = append(c.gatewayListeners[gatewayID], gatewayListener)
		}
	}

	for _, key := range c.sortedListenerKeys() {
		c.validateHostnameClaims(c.listenerGroups[key])
	}
	c.validateListenerPortOwnership()
}

func (c *compilation) validListenerCertificate(gatewayID string, listener gatewayv1.Listener) bool {
	if listener.CertificateRef == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidSpec, fmt.Sprintf("gateway %q HTTPS listener %q must reference a certificate", gatewayID, listener.Name))
		return false
	}
	if _, exists := c.certificates[listener.CertificateRef]; !exists {
		c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonReferenceNotFound, fmt.Sprintf("gateway %q HTTPS listener %q references missing certificate %q", gatewayID, listener.Name, listener.CertificateRef))
		return false
	}
	if !c.validCertificates[listener.CertificateRef] {
		c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidReference, fmt.Sprintf("gateway %q HTTPS listener %q references invalid certificate %q", gatewayID, listener.Name, listener.CertificateRef))
		return false
	}
	return true
}

func (c *compilation) validateHostnameClaims(group *listenerGroup) {
	for i, first := range group.claims {
		for _, second := range group.claims[i+1:] {
			if !hostnameutil.Overlaps(first.hostname, second.hostname) {
				continue
			}
			message := fmt.Sprintf("listener %s hostname ranges %q and %q overlap", listenerName(group.key), first.hostname, second.hostname)
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, first.gatewayID, ReasonConflict, message)
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, second.gatewayID, ReasonConflict, message)
		}
	}
}

func (c *compilation) validateListenerPortOwnership() {
	keys := c.sortedListenerKeys()
	for i, first := range keys {
		for _, second := range keys[i+1:] {
			if first.address != second.address || first.port != second.port || first.protocol == second.protocol {
				continue
			}
			message := fmt.Sprintf("port %d is configured for both %s and %s gateways", first.port, first.protocol, second.protocol)
			for _, listener := range c.listenerGroups[first].gateways {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listener.gatewayID, ReasonConflict, message)
			}
			for _, listener := range c.listenerGroups[second].gateways {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listener.gatewayID, ReasonConflict, message)
			}
		}
	}
}

func (c *compilation) buildListeners(filters map[listenerKey]listenerFilterConfig) []*listenerv3.Listener {
	listeners := make([]*listenerv3.Listener, 0, len(c.listenerGroups))
	for _, key := range c.sortedListenerKeys() {
		httpFilters, err := buildHTTPFilters(filters[key])
		if err != nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listenerName(key), ReasonCompileFailed, err.Error())
			continue
		}
		manager := &hcmv3.HttpConnectionManager{
			CodecType:             hcmv3.HttpConnectionManager_AUTO,
			StatPrefix:            listenerName(key),
			StripMatchingHostPort: true,
			RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{Rds: &hcmv3.Rds{
				ConfigSource:    adsConfigSource(),
				RouteConfigName: routeConfigName(key),
			}},
			HttpFilters: httpFilters,
		}
		if err := manager.ValidateAll(); err != nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listenerName(key), ReasonCompileFailed, fmt.Sprintf("validate HTTP connection manager for listener %s: %v", listenerName(key), err))
			continue
		}
		hcm, err := anypb.New(manager)
		if err != nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listenerName(key), ReasonCompileFailed, fmt.Sprintf("encode HTTP connection manager for listener %s: %v", listenerName(key), err))
			continue
		}

		listener := &listenerv3.Listener{Name: listenerName(key), Address: socketAddress(key.address, key.port)}
		if key.protocol == gatewayv1.ProtocolHTTP {
			listener.FilterChains = []*listenerv3.FilterChain{httpFilterChain(hcm)}
		} else if err := c.configureHTTPSListener(listener, c.listenerGroups[key], hcm); err != nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listenerName(key), ReasonCompileFailed, err.Error())
			continue
		}
		listeners = append(listeners, listener)
	}
	return listeners
}

func (c *compilation) configureHTTPSListener(listener *listenerv3.Listener, group *listenerGroup, hcm *anypb.Any) error {
	inspector, err := anypb.New(&tlsinspectorv3.TlsInspector{})
	if err != nil {
		return fmt.Errorf("encode TLS inspector for listener %s: %w", listener.Name, err)
	}
	listener.ListenerFilters = []*listenerv3.ListenerFilter{{
		Name:       tlsInspectorFilterName,
		ConfigType: &listenerv3.ListenerFilter_TypedConfig{TypedConfig: inspector},
	}}

	gateways := slices.Clone(group.gateways)
	slices.SortFunc(gateways, func(a, b gatewayListener) int {
		if result := cmp.Compare(a.gatewayID, b.gatewayID); result != 0 {
			return result
		}
		return cmp.Compare(a.hosts[0], b.hosts[0])
	})
	for _, gateway := range gateways {
		filterChain, err := buildHTTPSFilterChain(listener.Name, gateway, c.certificates[gateway.certificateRef], hcm)
		if err != nil {
			return err
		}
		if gateway.hosts[0] == "*" {
			listener.DefaultFilterChain = filterChain
		} else {
			listener.FilterChains = append(listener.FilterChains, filterChain)
		}
	}
	return nil
}

func buildHTTPSFilterChain(listenerName string, gateway gatewayListener, certificate *gatewayv1.Certificate, hcm *anypb.Any) (*listenerv3.FilterChain, error) {
	tlsContext := &tlsv3.DownstreamTlsContext{CommonTlsContext: &tlsv3.CommonTlsContext{
		TlsCertificates: []*tlsv3.TlsCertificate{{
			CertificateChain: inlineStringDataSource(certificate.Spec.CertificatePEM),
			PrivateKey:       inlineStringDataSource(certificate.Spec.PrivateKeyPEM),
		}},
		AlpnProtocols: []string{"h2", "http/1.1"},
	}}
	if err := tlsContext.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate TLS context for gateway %q on listener %s: %w", gateway.gatewayID, listenerName, err)
	}
	typedTLSContext, err := anypb.New(tlsContext)
	if err != nil {
		return nil, fmt.Errorf("encode TLS context for gateway %q on listener %s: %w", gateway.gatewayID, listenerName, err)
	}
	filterChain := httpFilterChain(hcm)
	filterChain.Name = listenerName + "/gateway/" + gateway.gatewayID + "/" + gateway.hosts[0]
	filterChain.TransportSocket = &corev3.TransportSocket{
		Name:       tlsTransportSocketName,
		ConfigType: &corev3.TransportSocket_TypedConfig{TypedConfig: typedTLSContext},
	}
	if gateway.hosts[0] != "*" {
		filterChain.FilterChainMatch = &listenerv3.FilterChainMatch{
			ServerNames:       slices.Clone(gateway.hosts),
			TransportProtocol: tlsTransportProtocol,
		}
	}
	return filterChain, nil
}

func httpFilterChain(hcm *anypb.Any) *listenerv3.FilterChain {
	return &listenerv3.FilterChain{Filters: []*listenerv3.Filter{{
		Name:       httpConnectionManagerFilterName,
		ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: hcm},
	}}}
}

func inlineStringDataSource(value string) *corev3.DataSource {
	return &corev3.DataSource{Specifier: &corev3.DataSource_InlineString{InlineString: value}}
}

func buildHTTPFilters(config listenerFilterConfig) ([]*hcmv3.HttpFilter, error) {
	filters := make([]*hcmv3.HttpFilter, 0, 3)
	if config.ipRestriction != nil {
		filter, err := buildIPRestrictionHTTPFilter(config.ipRestriction)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	if config.rateLimit != nil {
		filter, err := buildRateLimitHTTPFilter(config.rateLimit)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	router, err := anypb.New(&routerv3.Router{})
	if err != nil {
		return nil, fmt.Errorf("encode Envoy router filter: %w", err)
	}
	filters = append(filters, &hcmv3.HttpFilter{
		Name:       httpRouterFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: router},
	})
	return filters, nil
}

func (c *compilation) sortedListenerKeys() []listenerKey {
	keys := slices.Collect(maps.Keys(c.listenerGroups))
	slices.SortFunc(keys, compareListenerKeys)
	return keys
}

func compareListenerKeys(a, b listenerKey) int {
	if a.address != b.address {
		return cmp.Compare(a.address, b.address)
	}
	if a.port != b.port {
		return cmp.Compare(a.port, b.port)
	}
	return cmp.Compare(a.protocol, b.protocol)
}

func listenerName(key listenerKey) string {
	return fmt.Sprintf("ingate/%s-%d", strings.ToLower(string(key.protocol)), key.port)
}

func routeConfigName(key listenerKey) string {
	return listenerName(key) + "/routes"
}

func hostnameCoveredByListener(hostname, listenerHostname string) bool {
	if listenerHostname == "*" {
		return true
	}
	if hostname == "*" {
		return false
	}
	if !strings.HasPrefix(listenerHostname, "*.") {
		return hostname == listenerHostname
	}
	listenerSuffix := strings.TrimPrefix(listenerHostname, "*")
	if !strings.HasPrefix(hostname, "*.") {
		return strings.HasSuffix(hostname, listenerSuffix)
	}
	return strings.HasSuffix(strings.TrimPrefix(hostname, "*"), listenerSuffix)
}
