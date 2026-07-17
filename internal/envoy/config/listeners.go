package config

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	defaultBindAddress              = "0.0.0.0"
	httpConnectionManagerFilterName = "envoy.filters.network.http_connection_manager"
	httpRouterFilterName            = "envoy.filters.http.router"
)

type listenerKey struct {
	address  string
	port     int
	protocol gatewayv1.Protocol
}

type listenerGroup struct {
	key    listenerKey
	claims []hostnameClaim
}

type gatewayListener struct {
	key   listenerKey
	hosts []string
}

type hostnameClaim struct {
	gatewayID string
	hostname  string
}

type gatewayListenerDeclaration struct {
	key       listenerKey
	supported bool
}

func (c *compileContext) buildListenerGroups() {
	// 一套 Ingate 中所有 Gateway 共享同一组 Envoy Listener，端口和协议相同的入口必须合并
	// Hostname 所有权在合并后统一检查，避免两个逻辑 Gateway 抢占同一个请求域
	gatewayIDs := slices.Sorted(maps.Keys(c.gateways))
	protocolOwners := make(map[int]map[gatewayv1.Protocol]map[string]bool)

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
			declaration := gatewayListenerDeclaration{key: key}
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
			if listener.Port >= 1 && listener.Port <= 65535 &&
				(listener.Protocol == gatewayv1.ProtocolHTTP || listener.Protocol == gatewayv1.ProtocolHTTPS) {
				if protocolOwners[listener.Port] == nil {
					protocolOwners[listener.Port] = make(map[gatewayv1.Protocol]map[string]bool)
				}
				if protocolOwners[listener.Port][listener.Protocol] == nil {
					protocolOwners[listener.Port][listener.Protocol] = make(map[string]bool)
				}
				protocolOwners[listener.Port][listener.Protocol][gatewayID] = true
			}

			switch listener.Protocol {
			case gatewayv1.ProtocolHTTP:
				declaration.supported = listener.Port >= 1 && listener.Port <= 65535 && !duplicatePort
			case gatewayv1.ProtocolHTTPS:
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindGateway,
					gatewayID,
					ReasonUnsupported,
					fmt.Sprintf("gateway %q listener %q uses unsupported HTTPS", gatewayID, listener.Name),
				)
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

	for port, protocols := range protocolOwners {
		if len(protocols) < 2 {
			continue
		}
		owners := make(map[string]bool)
		for _, protocolOwners := range protocols {
			for gatewayID := range protocolOwners {
				owners[gatewayID] = true
			}
		}
		for _, gatewayID := range slices.Sorted(maps.Keys(owners)) {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindGateway,
				gatewayID,
				ReasonConflict,
				fmt.Sprintf("listener port %d is declared with conflicting HTTP and HTTPS protocols", port),
			)
		}
	}

	for _, key := range c.sortedListenerKeys() {
		c.validateHostnameClaims(c.listenerGroups[key])
	}
}

func (c *compileContext) buildGatewayListenerClaims(
	gateway *gatewayv1.Gateway,
	declarations map[string]gatewayListenerDeclaration,
) {
	hostsByKey := make(map[listenerKey]map[string]bool)
	keys := make(map[listenerKey]bool)
	for _, declaration := range declarations {
		if !declaration.supported {
			continue
		}
		keys[declaration.key] = true
		hostsByKey[declaration.key] = make(map[string]bool)
	}

	for _, binding := range gateway.Spec.HostBindings {
		hostname, ok := normalizeHostname(binding.Hostname)
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
		if binding.TLS != nil {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindGateway,
				gateway.Name,
				ReasonUnsupported,
				fmt.Sprintf("gateway %q hostname %q uses unsupported certificateRef", gateway.Name, hostname),
			)
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
		c.gatewayListeners[gateway.Name] = append(c.gatewayListeners[gateway.Name], gatewayListener{
			key:   key,
			hosts: hosts,
		})
		group := c.listenerGroups[key]
		if group == nil {
			group = &listenerGroup{key: key}
			c.listenerGroups[key] = group
		}
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

func (c *compileContext) validateHostnameClaims(group *listenerGroup) {
	slices.SortFunc(group.claims, func(a, b hostnameClaim) int {
		if a.gatewayID != b.gatewayID {
			return cmp.Compare(a.gatewayID, b.gatewayID)
		}
		return cmp.Compare(a.hostname, b.hostname)
	})
	for i, first := range group.claims {
		for _, second := range group.claims[i+1:] {
			if !hostnamesOverlap(first.hostname, second.hostname) {
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

func (c *compileContext) buildListeners(policies map[listenerKey]listenerPolicyConfig) []*listenerv3.Listener {
	keys := c.sortedListenerKeys()
	listeners := make([]*listenerv3.Listener, 0, len(keys))
	for _, key := range keys {
		httpFilters, err := c.buildHTTPFilters(policies[key])
		if err != nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listenerName(key), ReasonCompileFailed, err.Error())
			continue
		}

		hcm, err := anypb.New(&hcmv3.HttpConnectionManager{
			CodecType:  hcmv3.HttpConnectionManager_AUTO,
			StatPrefix: listenerName(key),
			RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
				Rds: &hcmv3.Rds{
					ConfigSource:    adsConfigSource(),
					RouteConfigName: routeConfigName(key),
				},
			},
			HttpFilters: httpFilters,
		})
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

		listeners = append(listeners, &listenerv3.Listener{
			Name:    listenerName(key),
			Address: socketAddress(key.address, key.port),
			FilterChains: []*listenerv3.FilterChain{
				{
					Filters: []*listenerv3.Filter{
						{
							Name: httpConnectionManagerFilterName,
							ConfigType: &listenerv3.Filter_TypedConfig{
								TypedConfig: hcm,
							},
						},
					},
				},
			},
		})
	}
	return listeners
}

func (c *compileContext) buildHTTPFilters(policies listenerPolicyConfig) ([]*hcmv3.HttpFilter, error) {
	filters := make([]*hcmv3.HttpFilter, 0, 3)
	if policies.accessControl != nil {
		filter, err := buildAccessControlHTTPFilter(policies.accessControl)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	if policies.rateLimit != nil {
		filter, err := buildRateLimitHTTPFilter(policies.rateLimit)
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
		Name: httpRouterFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: router,
		},
	})
	return filters, nil
}

func (c *compileContext) sortedListenerKeys() []listenerKey {
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

func normalizeHostname(hostname string) (string, bool) {
	if hostname == "" || hostname == "*" {
		return "*", true
	}
	if strings.TrimSpace(hostname) != hostname {
		return "", false
	}
	hostname = strings.ToLower(hostname)
	if strings.HasPrefix(hostname, "*.") {
		if !validDNSName(strings.TrimPrefix(hostname, "*.")) {
			return "", false
		}
		return hostname, true
	}
	return hostname, validDNSName(hostname)
}

func validDNSName(hostname string) bool {
	if hostname == "" || len(hostname) > 253 {
		return false
	}
	for label := range strings.SplitSeq(hostname, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for i, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
			if (i == 0 || i == len(label)-1) && r == '-' {
				return false
			}
		}
	}
	return true
}

func hostnamesOverlap(first, second string) bool {
	if first == "*" || second == "*" {
		return true
	}
	firstWildcard := strings.HasPrefix(first, "*.")
	secondWildcard := strings.HasPrefix(second, "*.")
	switch {
	case !firstWildcard && !secondWildcard:
		return first == second
	case firstWildcard && secondWildcard:
		firstSuffix := strings.TrimPrefix(first, "*")
		secondSuffix := strings.TrimPrefix(second, "*")
		return strings.HasSuffix(firstSuffix, secondSuffix) || strings.HasSuffix(secondSuffix, firstSuffix)
	case firstWildcard:
		return strings.HasSuffix(second, strings.TrimPrefix(first, "*"))
	default:
		return strings.HasSuffix(first, strings.TrimPrefix(second, "*"))
	}
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
