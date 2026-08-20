package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/types/known/anypb"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultBindAddress              = "0.0.0.0"
	httpConnectionManagerFilterName = "envoy.filters.network.http_connection_manager"
)

type listenerKey struct {
	address  string
	port     int
	protocol gatewayv1.Protocol
}

func (c *compilation) buildListeners(
	groups map[listenerKey]*listenerGroup,
	filters map[listenerKey]listenerFilterConfig,
) []*listenerv3.Listener {
	listeners := make([]*listenerv3.Listener, 0, len(groups))
	for _, key := range sortedListenerKeys(groups) {
		requestLog, err := buildHTTPAccessLog()
		if err != nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listenerName(key), ReasonCompileFailed, fmt.Sprintf("encode access log for listener %s: %v", listenerName(key), err))
			continue
		}
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
			AccessLog:   []*accesslogv3.AccessLog{requestLog},
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
		} else if err := c.configureHTTPSListener(listener, groups[key], hcm); err != nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listenerName(key), ReasonCompileFailed, err.Error())
			continue
		}
		listeners = append(listeners, listener)
	}
	return listeners
}

func httpFilterChain(hcm *anypb.Any) *listenerv3.FilterChain {
	return &listenerv3.FilterChain{Filters: []*listenerv3.Filter{{
		Name:       httpConnectionManagerFilterName,
		ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: hcm},
	}}}
}

func sortedListenerKeys(groups map[listenerKey]*listenerGroup) []listenerKey {
	keys := slices.Collect(maps.Keys(groups))
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
