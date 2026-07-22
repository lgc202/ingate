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
	pluginaiproxy "github.com/lgc202/ingate/pkg/plugin/aiproxy"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
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

type gatewayListenerDeclaration struct {
	key            listenerKey
	certificateRef string
	supported      bool
}

func (c *compilation) buildListenerGroups() {
	// 一套 Ingate 中所有 Gateway 共享同一组 Envoy Listener，端口和协议相同的入口必须合并
	// Hostname 所有权在合并后统一检查，避免两个逻辑 Gateway 抢占同一个请求域
	gatewayIDs := slices.Sorted(maps.Keys(c.gateways))

	for _, gatewayID := range gatewayIDs {
		gateway := c.gateways[gatewayID]
		if !gateway.Spec.Enabled {
			continue
		}
		if len(gateway.Spec.Listeners) == 0 {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindGateway,
				gatewayID,
				ReasonInvalidSpec,
				fmt.Sprintf("gateway %q must declare at least one listener", gatewayID),
			)
			continue
		}

		declarations := make(map[string]gatewayListenerDeclaration, len(gateway.Spec.Listeners))
		ports := make(map[int]string, len(gateway.Spec.Listeners))
		for _, listener := range gateway.Spec.Listeners {
			if listener.Name == "" {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindGateway,
					gatewayID,
					ReasonInvalidSpec,
					fmt.Sprintf("gateway %q has a listener without a name", gatewayID),
				)
				continue
			}
			if _, ok := declarations[listener.Name]; ok {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindGateway,
					gatewayID,
					ReasonConflict,
					fmt.Sprintf("gateway %q has duplicate listener %q", gatewayID, listener.Name),
				)
				continue
			}

			key := listenerKey{
				address:  defaultBindAddress,
				port:     listener.Port,
				protocol: listener.Protocol,
			}
			declaration := gatewayListenerDeclaration{
				key:            key,
				certificateRef: listener.CertificateRef,
			}
			duplicatePort := false
			if listener.Port < 1 || listener.Port > 65535 {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindGateway,
					gatewayID,
					ReasonInvalidSpec,
					fmt.Sprintf("gateway %q listener %q port must be between 1 and 65535", gatewayID, listener.Name),
				)
			} else {
				if firstListener, exists := ports[listener.Port]; exists {
					c.addDiagnostic(
						SeverityError,
						gatewayv1.KindGateway,
						gatewayID,
						ReasonConflict,
						fmt.Sprintf(
							"gateway %q listeners %q and %q use the same port %d",
							gatewayID,
							firstListener,
							listener.Name,
							listener.Port,
						),
					)
					duplicatePort = true
				} else {
					ports[listener.Port] = listener.Name
				}
			}
			validPort := listener.Port >= 1 && listener.Port <= 65535 && !duplicatePort
			switch listener.Protocol {
			case gatewayv1.ProtocolHTTP:
				if listener.CertificateRef != "" {
					c.addDiagnostic(
						SeverityError,
						gatewayv1.KindGateway,
						gatewayID,
						ReasonInvalidSpec,
						fmt.Sprintf("gateway %q HTTP listener %q must not reference a certificate", gatewayID, listener.Name),
					)
					break
				}
				declaration.supported = validPort
			case gatewayv1.ProtocolHTTPS:
				if listener.CertificateRef == "" {
					c.addDiagnostic(
						SeverityError,
						gatewayv1.KindGateway,
						gatewayID,
						ReasonInvalidSpec,
						fmt.Sprintf("gateway %q HTTPS listener %q must reference a certificate", gatewayID, listener.Name),
					)
					break
				}
				if _, exists := c.certificates[listener.CertificateRef]; !exists {
					c.addDiagnostic(
						SeverityError,
						gatewayv1.KindGateway,
						gatewayID,
						ReasonReferenceNotFound,
						fmt.Sprintf(
							"gateway %q HTTPS listener %q references missing certificate %q",
							gatewayID,
							listener.Name,
							listener.CertificateRef,
						),
					)
					break
				}
				if !c.validCertificates[listener.CertificateRef] {
					c.addDiagnostic(
						SeverityError,
						gatewayv1.KindGateway,
						gatewayID,
						ReasonInvalidSpec,
						fmt.Sprintf(
							"gateway %q HTTPS listener %q references invalid certificate %q",
							gatewayID,
							listener.Name,
							listener.CertificateRef,
						),
					)
					break
				}
				declaration.supported = validPort
			default:
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindGateway,
					gatewayID,
					ReasonUnsupported,
					fmt.Sprintf("gateway %q listener %q uses unsupported protocol %q", gatewayID, listener.Name, listener.Protocol),
				)
			}
			declarations[listener.Name] = declaration
		}

		c.buildGatewayListenerClaims(gateway, declarations)
	}

	for _, key := range c.sortedListenerKeys() {
		c.validateHostnameClaims(c.listenerGroups[key])
	}
	c.validateListenerPortOwnership()
}

func (c *compilation) buildGatewayListenerClaims(
	gateway *gatewayv1.Gateway,
	declarations map[string]gatewayListenerDeclaration,
) {
	hostsByKey := make(map[listenerKey]map[string]bool)
	certificateRefs := make(map[listenerKey]string)
	keys := make(map[listenerKey]bool)
	for _, declaration := range declarations {
		if !declaration.supported {
			continue
		}
		keys[declaration.key] = true
		hostsByKey[declaration.key] = make(map[string]bool)
		certificateRefs[declaration.key] = declaration.certificateRef
	}

	for _, binding := range gateway.Spec.HostBindings {
		hostname, ok := hostnameutil.Normalize(binding.Hostname)
		if !ok {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindGateway,
				gateway.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("gateway %q has invalid hostname %q", gateway.Name, binding.Hostname),
			)
			continue
		}
		if len(binding.ListenerRefs) == 0 {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindGateway,
				gateway.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("gateway %q hostname %q must reference at least one listener", gateway.Name, hostname),
			)
			continue
		}

		bindingKeys := make(map[listenerKey]bool)
		seenRefs := make(map[string]bool, len(binding.ListenerRefs))
		for _, ref := range binding.ListenerRefs {
			if ref == "" {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindGateway,
					gateway.Name,
					ReasonInvalidSpec,
					fmt.Sprintf("gateway %q hostname %q has an empty listener reference", gateway.Name, hostname),
				)
				continue
			}
			if seenRefs[ref] {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindGateway,
					gateway.Name,
					ReasonConflict,
					fmt.Sprintf("gateway %q hostname %q references listener %q more than once", gateway.Name, hostname, ref),
				)
				continue
			}
			seenRefs[ref] = true

			declaration, exists := declarations[ref]
			if !exists {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindGateway,
					gateway.Name,
					ReasonReferenceNotFound,
					fmt.Sprintf("gateway %q hostname %q references unknown listener %q", gateway.Name, hostname, ref),
				)
				continue
			}
			if declaration.supported {
				bindingKeys[declaration.key] = true
			}
		}
		for key := range bindingKeys {
			if hostsByKey[key][hostname] {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindGateway,
					gateway.Name,
					ReasonConflict,
					fmt.Sprintf("gateway %q assigns hostname %q to the same listener more than once", gateway.Name, hostname),
				)
				continue
			}
			hostsByKey[key][hostname] = true
		}
	}

	for key := range keys {
		// 某个 Listener 没有显式 HostBinding 时，它接管该入口上的全部 Host
		if len(hostsByKey[key]) == 0 {
			hostsByKey[key]["*"] = true
		}
		hosts := slices.Sorted(maps.Keys(hostsByKey[key]))
		compiledListener := gatewayListener{
			key:            key,
			gatewayID:      gateway.Name,
			hosts:          hosts,
			certificateRef: certificateRefs[key],
		}
		c.gatewayListeners[gateway.Name] = append(c.gatewayListeners[gateway.Name], compiledListener)
		group := c.listenerGroups[key]
		if group == nil {
			group = &listenerGroup{key: key}
			c.listenerGroups[key] = group
		}
		group.gateways = append(group.gateways, compiledListener)
		for _, hostname := range hosts {
			group.claims = append(group.claims, hostnameClaim{
				gatewayID: gateway.Name,
				hostname:  hostname,
			})
		}
	}
	slices.SortFunc(c.gatewayListeners[gateway.Name], func(a, b gatewayListener) int {
		return compareListenerKeys(a.key, b.key)
	})
}

func (c *compilation) validateHostnameClaims(group *listenerGroup) {
	slices.SortFunc(group.claims, func(a, b hostnameClaim) int {
		if a.gatewayID != b.gatewayID {
			return cmp.Compare(a.gatewayID, b.gatewayID)
		}
		return cmp.Compare(a.hostname, b.hostname)
	})
	for i, first := range group.claims {
		for _, second := range group.claims[i+1:] {
			if !hostnameutil.Overlaps(first.hostname, second.hostname) {
				continue
			}
			message := fmt.Sprintf(
				"listener %s has conflicting hostname ownership between gateway %q hostname %q and gateway %q hostname %q",
				listenerName(group.key),
				first.gatewayID,
				first.hostname,
				second.gatewayID,
				second.hostname,
			)
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, first.gatewayID, ReasonConflict, message)
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, second.gatewayID, ReasonConflict, message)
		}
	}
}

func (c *compilation) validateListenerPortOwnership() {
	keys := c.sortedListenerKeys()
	for i, first := range keys {
		for _, second := range keys[i+1:] {
			if first.address != second.address || first.port != second.port {
				continue
			}
			if first.protocol == second.protocol {
				continue
			}

			message := fmt.Sprintf(
				"port %d is configured for both %s and %s gateways",
				first.port,
				first.protocol,
				second.protocol,
			)
			for _, gateway := range c.listenerGroups[first].gateways {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gateway.gatewayID, ReasonConflict, message)
			}
			for _, gateway := range c.listenerGroups[second].gateways {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gateway.gatewayID, ReasonConflict, message)
			}
		}
	}
}

func (c *compilation) buildListeners(plugins map[listenerKey]listenerPluginConfig) []*listenerv3.Listener {
	keys := c.sortedListenerKeys()
	listeners := make([]*listenerv3.Listener, 0, len(keys))
	for _, key := range keys {
		httpFilters, err := c.buildHTTPFilters(plugins[key])
		if err != nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listenerName(key), ReasonCompileFailed, err.Error())
			continue
		}

		manager := &hcmv3.HttpConnectionManager{
			CodecType:             hcmv3.HttpConnectionManager_AUTO,
			StatPrefix:            listenerName(key),
			StripMatchingHostPort: true,
			RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
				Rds: &hcmv3.Rds{
					ConfigSource:    adsConfigSource(),
					RouteConfigName: routeConfigName(key),
				},
			},
			HttpFilters: httpFilters,
		}
		if err := manager.ValidateAll(); err != nil {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindGateway,
				listenerName(key),
				ReasonCompileFailed,
				fmt.Sprintf("validate HTTP connection manager for listener %s: %v", listenerName(key), err),
			)
			continue
		}
		hcm, err := anypb.New(manager)
		if err != nil {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindGateway,
				listenerName(key),
				ReasonCompileFailed,
				fmt.Sprintf("encode HTTP connection manager for listener %s: %v", listenerName(key), err),
			)
			continue
		}

		listener := &listenerv3.Listener{
			Name:    listenerName(key),
			Address: socketAddress(key.address, key.port),
		}
		if aiProxy := plugins[key].aiProxy; aiProxy != nil && len(aiProxy.Routes) > 0 {
			// Listener 软限制需要高于 Wasm 业务上限，确保首个越界字节到达插件并触发统一 502
			listener.PerConnectionBufferLimitBytes = wrapperspb.UInt32(pluginaiproxy.ResponseBufferLimitBytes)
		}
		switch key.protocol {
		case gatewayv1.ProtocolHTTP:
			listener.FilterChains = []*listenerv3.FilterChain{httpFilterChain(hcm)}
		case gatewayv1.ProtocolHTTPS:
			if err := c.configureHTTPSListener(listener, c.listenerGroups[key], hcm); err != nil {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listenerName(key), ReasonCompileFailed, err.Error())
				continue
			}
		default:
			continue
		}
		listeners = append(listeners, listener)
	}
	return listeners
}

func (c *compilation) configureHTTPSListener(
	listener *listenerv3.Listener,
	group *listenerGroup,
	hcm *anypb.Any,
) error {
	// TLS Inspector 先提取 SNI，同一 8443 Listener 再按逻辑 Gateway 的 Host 所有权选择证书和 HCM filter chain
	inspectorConfig := &tlsinspectorv3.TlsInspector{}
	if err := inspectorConfig.ValidateAll(); err != nil {
		return fmt.Errorf("validate TLS inspector for listener %s: %w", listener.Name, err)
	}
	inspector, err := anypb.New(inspectorConfig)
	if err != nil {
		return fmt.Errorf("encode TLS inspector for listener %s: %w", listener.Name, err)
	}
	listener.ListenerFilters = []*listenerv3.ListenerFilter{
		{
			Name: tlsInspectorFilterName,
			ConfigType: &listenerv3.ListenerFilter_TypedConfig{
				TypedConfig: inspector,
			},
		},
	}

	gateways := slices.Clone(group.gateways)
	slices.SortFunc(gateways, func(a, b gatewayListener) int {
		return cmp.Compare(a.gatewayID, b.gatewayID)
	})
	for _, gateway := range gateways {
		certificate := c.certificates[gateway.certificateRef]
		filterChain, err := buildHTTPSFilterChain(listener.Name, gateway, certificate, hcm)
		if err != nil {
			return err
		}
		if slices.Contains(gateway.hosts, "*") {
			// Envoy 不接受 ServerNames=["*"]，无 Host 限制的 Gateway 必须使用 DefaultFilterChain
			listener.DefaultFilterChain = filterChain
			continue
		}
		listener.FilterChains = append(listener.FilterChains, filterChain)
	}
	return nil
}

func buildHTTPSFilterChain(
	listenerName string,
	gateway gatewayListener,
	certificate *gatewayv1.Certificate,
	hcm *anypb.Any,
) (*listenerv3.FilterChain, error) {
	// Certificate 是声明式事实，本轮直接随 LDS 下发 PEM；后续需要独立密钥轮转时再引入 SDS
	tlsContext := &tlsv3.DownstreamTlsContext{
		CommonTlsContext: &tlsv3.CommonTlsContext{
			TlsCertificates: []*tlsv3.TlsCertificate{
				{
					CertificateChain: inlineStringDataSource(certificate.Spec.CertificatePEM),
					PrivateKey:       inlineStringDataSource(certificate.Spec.PrivateKeyPEM),
				},
			},
			AlpnProtocols: []string{"h2", "http/1.1"},
		},
	}
	if err := tlsContext.ValidateAll(); err != nil {
		return nil, fmt.Errorf(
			"validate TLS context for gateway %q on listener %s: %w",
			gateway.gatewayID,
			listenerName,
			err,
		)
	}
	typedTLSContext, err := anypb.New(tlsContext)
	if err != nil {
		return nil, fmt.Errorf(
			"encode TLS context for gateway %q on listener %s: %w",
			gateway.gatewayID,
			listenerName,
			err,
		)
	}

	filterChain := httpFilterChain(hcm)
	filterChain.Name = listenerName + "/gateway/" + gateway.gatewayID
	filterChain.TransportSocket = &corev3.TransportSocket{
		Name: tlsTransportSocketName,
		ConfigType: &corev3.TransportSocket_TypedConfig{
			TypedConfig: typedTLSContext,
		},
	}
	if !slices.Contains(gateway.hosts, "*") {
		filterChain.FilterChainMatch = &listenerv3.FilterChainMatch{
			ServerNames:       slices.Clone(gateway.hosts),
			TransportProtocol: tlsTransportProtocol,
		}
	}
	return filterChain, nil
}

func httpFilterChain(hcm *anypb.Any) *listenerv3.FilterChain {
	return &listenerv3.FilterChain{
		Filters: []*listenerv3.Filter{
			{
				Name: httpConnectionManagerFilterName,
				ConfigType: &listenerv3.Filter_TypedConfig{
					TypedConfig: hcm,
				},
			},
		},
	}
}

func inlineStringDataSource(value string) *corev3.DataSource {
	return &corev3.DataSource{
		Specifier: &corev3.DataSource_InlineString{InlineString: value},
	}
}

func (c *compilation) buildHTTPFilters(plugins listenerPluginConfig) ([]*hcmv3.HttpFilter, error) {
	filters := make([]*hcmv3.HttpFilter, 0, 5)
	if plugins.accessControl != nil {
		filter, err := buildAccessControlHTTPFilter(plugins.accessControl)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	if plugins.rateLimit != nil {
		filter, err := buildRateLimitHTTPFilter(plugins.rateLimit)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	if plugins.tokenQuota != nil {
		filter, err := buildTokenQuotaHTTPFilter(plugins.tokenQuota)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	aiProxy := plugins.aiProxy
	if aiProxy == nil {
		aiProxy = &pluginaiproxy.PluginConfig{Routes: []pluginaiproxy.RouteConfig{}}
	}
	filter, err := buildAIProxyHTTPFilter(aiProxy)
	if err != nil {
		return nil, err
	}
	filters = append(filters, filter)

	routerConfig := &routerv3.Router{}
	if err := routerConfig.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate Envoy router filter: %w", err)
	}
	router, err := anypb.New(routerConfig)
	if err != nil {
		return nil, fmt.Errorf("encode Envoy router filter: %w", err)
	}
	filters = append(filters, &hcmv3.HttpFilter{
		Name: httpRouterFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: router,
		},
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
	listenerWildcard := strings.HasPrefix(listenerHostname, "*.")
	if !listenerWildcard {
		return hostname == listenerHostname
	}
	listenerSuffix := strings.TrimPrefix(listenerHostname, "*")
	if !strings.HasPrefix(hostname, "*.") {
		return strings.HasSuffix(hostname, listenerSuffix)
	}
	hostnameSuffix := strings.TrimPrefix(hostname, "*")
	return strings.HasSuffix(hostnameSuffix, listenerSuffix)
}
