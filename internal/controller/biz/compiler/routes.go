package compiler

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/types/known/anypb"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// preparedRoute 保存一条 Route 进入 Listener 挂载阶段所需的已校验数据。
type preparedRoute struct {
	routeID           string
	gatewayIDs        []string
	hostnames         []string
	explicitHostnames bool
	entries           []routeEntry
	accessConfig      *anypb.Any
}

func (c *compilation) buildRoutes(
	listenerGroups map[listenerKey]*listenerGroup,
	listenersByGateway map[string][]gatewayListener,
	compiledUpstreams map[string]bool,
) ([]*routev3.RouteConfiguration, []routeAttachment) {
	table := newRouteTable(len(listenerGroups))
	for _, routeID := range slices.Sorted(maps.Keys(c.routes)) {
		route := c.routes[routeID]
		if !route.Spec.Enabled {
			continue
		}
		prepared, ok := c.prepareRoute(route, compiledUpstreams)
		if !ok {
			continue
		}
		c.attachRoute(table, prepared, listenersByGateway)
	}
	return table.configurations(listenerGroups), table.sortedAttachments()
}

func (c *compilation) prepareRoute(
	route *gatewayv1.Route,
	compiledUpstreams map[string]bool,
) (preparedRoute, bool) {
	hostnames, hostnamesValid := c.routeHostnames(route)
	gatewayIDs, gatewaysValid := c.routeGatewayIDs(route)
	accessConfig, accessValid := c.routeAccessConfig(route)
	entries := c.buildRouteEntries(route, compiledUpstreams)
	if !hostnamesValid || !gatewaysValid || !accessValid || len(entries) == 0 {
		return preparedRoute{}, false
	}
	return preparedRoute{
		routeID:           route.Name,
		gatewayIDs:        gatewayIDs,
		hostnames:         hostnames,
		explicitHostnames: len(route.Spec.Hostnames) > 0,
		entries:           entries,
		accessConfig:      accessConfig,
	}, true
}

func (c *compilation) attachRoute(
	table *routeTable,
	route preparedRoute,
	listenersByGateway map[string][]gatewayListener,
) {
	attached := false
	hasEnabledGateway := false
	for _, gatewayID := range route.gatewayIDs {
		gateway, exists := c.gateways[gatewayID]
		if !exists {
			c.addRouteError(
				route.routeID,
				ReasonReferenceNotFound,
				fmt.Sprintf("route %q references missing gateway %q", route.routeID, gatewayID),
			)
			continue
		}
		if !gateway.Spec.Enabled {
			continue
		}
		hasEnabledGateway = true

		domainsByListener := c.routeDomainsByListener(
			route.routeID,
			gatewayID,
			route.hostnames,
			route.explicitHostnames,
			listenersByGateway,
		)
		if len(domainsByListener) == 0 {
			c.addRouteError(
				route.routeID,
				ReasonConflict,
				fmt.Sprintf(
					"route %q has no attachable listener on gateway %q",
					route.routeID,
					gatewayID,
				),
			)
			continue
		}

		for _, key := range sortedListenerKeySet(domainsByListener) {
			for _, domain := range slices.Sorted(maps.Keys(domainsByListener[key])) {
				for _, entry := range route.entries {
					previousRouteID, conflict := table.addEntry(
						key,
						domain,
						gatewayID,
						entry,
						route.accessConfig,
					)
					if conflict {
						message := fmt.Sprintf(
							"listener %s hostname %q has overlapping route matches with equal precedence in %q and %q",
							listenerName(key),
							domain,
							previousRouteID,
							route.routeID,
						)
						c.addRouteError(previousRouteID, ReasonConflict, message)
						c.addRouteError(route.routeID, ReasonConflict, message)
						continue
					}
					attached = true
				}
			}
		}
	}

	// 网关停用表示用户主动暂停入口，引用它的 Route 保留配置但暂不挂载。
	// 只有仍存在启用网关却无法挂载时，才属于需要阻断发布的配置错误。
	if hasEnabledGateway && !attached {
		c.addRouteError(
			route.routeID,
			ReasonConflict,
			fmt.Sprintf("route %q has no attachable listener", route.routeID),
		)
	}
}

func (c *compilation) routeHostnames(route *gatewayv1.Route) ([]string, bool) {
	if len(route.Spec.Hostnames) == 0 {
		return nil, true
	}
	if len(route.Spec.Hostnames) > routeconfig.MaxHostnames {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q declares too many hostnames", route.Name),
		)
		return nil, false
	}

	valid := true
	seenHostnames := make(map[string]bool, len(route.Spec.Hostnames))
	for _, hostnameValue := range route.Spec.Hostnames {
		hostname, ok := hostnameutil.Normalize(hostnameValue)
		if !ok || hostname == "*" {
			c.addRouteError(
				route.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q has invalid hostname %q", route.Name, hostnameValue),
			)
			valid = false
			continue
		}
		if seenHostnames[hostname] {
			c.addRouteError(
				route.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q declares hostname %q more than once", route.Name, hostname),
			)
			valid = false
			continue
		}
		seenHostnames[hostname] = true
	}
	return slices.Sorted(maps.Keys(seenHostnames)), valid
}

func (c *compilation) routeGatewayIDs(route *gatewayv1.Route) ([]string, bool) {
	if len(route.Spec.GatewayRefs) == 0 {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q must reference at least one gateway", route.Name),
		)
		return nil, false
	}
	if len(route.Spec.GatewayRefs) > routeconfig.MaxGatewayRefs {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q references too many gateways", route.Name),
		)
		return nil, false
	}

	valid := true
	seenGatewayIDs := make(map[string]bool, len(route.Spec.GatewayRefs))
	for _, gatewayID := range route.Spec.GatewayRefs {
		if !resourceconfig.IsCanonicalID(gatewayID) || seenGatewayIDs[gatewayID] {
			c.addRouteError(
				route.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q has an invalid or duplicate gateway reference %q", route.Name, gatewayID),
			)
			valid = false
			continue
		}
		seenGatewayIDs[gatewayID] = true
	}
	return slices.Sorted(maps.Keys(seenGatewayIDs)), valid
}

func (c *compilation) routeDomainsByListener(
	routeID string,
	gatewayID string,
	hostnames []string,
	explicitHostnames bool,
	listenersByGateway map[string][]gatewayListener,
) map[listenerKey]map[string]bool {
	domains := make(map[listenerKey]map[string]bool, len(listenersByGateway[gatewayID]))
	if !explicitHostnames {
		for _, listener := range listenersByGateway[gatewayID] {
			if domains[listener.key] == nil {
				domains[listener.key] = make(map[string]bool)
			}
			domains[listener.key][listener.hostname] = true
		}
		return domains
	}

	for _, hostname := range hostnames {
		matched := false
		for _, listener := range listenersByGateway[gatewayID] {
			if !hostnameCoveredByListener(hostname, listener.hostname) {
				continue
			}
			if domains[listener.key] == nil {
				domains[listener.key] = make(map[string]bool)
			}
			domains[listener.key][hostname] = true
			matched = true
		}
		if !matched {
			c.addRouteError(
				routeID,
				ReasonConflict,
				fmt.Sprintf(
					"route %q hostname %q does not belong to a listener on gateway %q",
					routeID,
					hostname,
					gatewayID,
				),
			)
		}
	}
	return domains
}

func sortedListenerKeySet(values map[listenerKey]map[string]bool) []listenerKey {
	keys := slices.Collect(maps.Keys(values))
	slices.SortFunc(keys, compareListenerKeys)
	return keys
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
