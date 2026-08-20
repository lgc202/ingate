package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/lgc202/ingate/internal/authz/filterconfig"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	envoyRouteNamePrefix  = "ingate-route"
	virtualHostNamePrefix = "ingate-vhost"
)

// routeAttachment 表示一条 Route 已成功展开到某个 Gateway Listener
// 内置治理策略只根据成功挂载结果生成执行索引
type routeAttachment struct {
	listenerKey listenerKey
	gatewayID   string
	routeID     string
	routes      []*routev3.Route
}

func (c *compilation) buildRoutes(
	listenerGroups map[listenerKey]*listenerGroup,
	listenersByGateway map[string][]gatewayListener,
	compiledUpstreams map[string]bool,
) ([]*routev3.RouteConfiguration, []routeAttachment) {
	routesByListener := make(map[listenerKey]map[string][]routeEntry, len(listenerGroups))
	matchOwners := make(map[listenerKey]map[string]map[string]routeEntry, len(listenerGroups))
	attachmentIndex := make(map[policyRouteKey]int)
	attachments := make([]routeAttachment, 0)

	for _, routeID := range slices.Sorted(maps.Keys(c.routes)) {
		route := c.routes[routeID]
		if !route.Spec.Enabled {
			continue
		}
		hostnames, explicitHostnames := c.routeHostnames(route)
		gatewayIDs := uniqueStrings(route.Spec.GatewayRefs)
		if len(gatewayIDs) == 0 {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q must reference at least one gateway", routeID))
			continue
		}
		accessConfig, accessValid := c.routeAccessConfig(route)
		if !accessValid {
			continue
		}
		entries := c.buildRouteEntries(route, compiledUpstreams)
		if len(entries) == 0 {
			continue
		}

		attached := false
		for _, gatewayID := range gatewayIDs {
			gateway, exists := c.gateways[gatewayID]
			if !exists {
				c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonReferenceNotFound, fmt.Sprintf("route %q references missing gateway %q", routeID, gatewayID))
				continue
			}
			if !gateway.Spec.Enabled {
				continue
			}
			domainsByListener := c.routeDomainsByListener(
				route,
				gatewayID,
				hostnames,
				explicitHostnames,
				listenersByGateway,
			)
			if len(domainsByListener) == 0 {
				c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonConflict, fmt.Sprintf("route %q has no attachable listener on gateway %q", routeID, gatewayID))
				continue
			}
			for _, key := range sortedListenerKeySet(domainsByListener) {
				if routesByListener[key] == nil {
					routesByListener[key] = make(map[string][]routeEntry)
				}
				if matchOwners[key] == nil {
					matchOwners[key] = make(map[string]map[string]routeEntry)
				}
				for _, domain := range slices.Sorted(maps.Keys(domainsByListener[key])) {
					if matchOwners[key][domain] == nil {
						matchOwners[key][domain] = make(map[string]routeEntry)
					}
					for _, entry := range entries {
						current := entry
						current.route = proto.Clone(entry.route).(*routev3.Route)
						if accessConfig != nil {
							if current.route.TypedPerFilterConfig == nil {
								current.route.TypedPerFilterConfig = make(map[string]*anypb.Any)
							}
							current.route.TypedPerFilterConfig[filterconfig.HTTPFilterName] = accessConfig
						}
						current.route.Name = envoyRouteName(gatewayID, routeID, entry.method, entry.variant)
						matchKey := routeMatchKey(current.route.Match)
						if previous, conflict := matchOwners[key][domain][matchKey]; conflict {
							message := fmt.Sprintf("listener %s hostname %q has the same route match in %q and %q", listenerName(key), domain, previous.routeID, routeID)
							c.addDiagnostic(SeverityError, gatewayv1.KindRoute, previous.routeID, ReasonConflict, message)
							c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonConflict, message)
							continue
						}
						matchOwners[key][domain][matchKey] = current
						routesByListener[key][domain] = append(routesByListener[key][domain], current)
						attached = true

						attachmentKey := policyRouteKey{listenerKey: key, gatewayID: gatewayID, routeID: routeID}
						index, exists := attachmentIndex[attachmentKey]
						if !exists {
							index = len(attachments)
							attachmentIndex[attachmentKey] = index
							attachments = append(attachments, routeAttachment{
								listenerKey: key,
								gatewayID:   gatewayID,
								routeID:     routeID,
							})
						}
						attachments[index].routes = append(attachments[index].routes, current.route)
					}
				}
			}
		}
		if !attached {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonConflict, fmt.Sprintf("route %q has no attachable listener", routeID))
		}
	}

	slices.SortFunc(attachments, compareRouteAttachments)
	configs := make([]*routev3.RouteConfiguration, 0, len(listenerGroups))
	for _, key := range sortedListenerKeys(listenerGroups) {
		virtualHosts := make([]*routev3.VirtualHost, 0, len(routesByListener[key]))
		for _, domain := range slices.Sorted(maps.Keys(routesByListener[key])) {
			entries := routesByListener[key][domain]
			slices.SortFunc(entries, compareRouteEntries)
			routes := make([]*routev3.Route, 0, len(entries))
			for _, entry := range entries {
				routes = append(routes, entry.route)
			}
			virtualHosts = append(virtualHosts, &routev3.VirtualHost{
				Name:    virtualHostName(key, domain),
				Domains: []string{domain},
				Routes:  routes,
			})
		}
		configs = append(configs, &routev3.RouteConfiguration{Name: routeConfigName(key), VirtualHosts: virtualHosts})
	}
	return configs, attachments
}

func (c *compilation) routeHostnames(route *gatewayv1.Route) ([]string, bool) {
	if len(route.Spec.Hostnames) == 0 {
		return nil, false
	}
	hostnames := make(map[string]bool, len(route.Spec.Hostnames))
	for _, value := range route.Spec.Hostnames {
		hostname, ok := hostnameutil.Normalize(value)
		if !ok || hostname == "*" {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q has invalid hostname %q", route.Name, value))
			continue
		}
		if hostnames[hostname] {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonConflict, fmt.Sprintf("route %q declares hostname %q more than once", route.Name, hostname))
			continue
		}
		hostnames[hostname] = true
	}
	return slices.Sorted(maps.Keys(hostnames)), true
}

func (c *compilation) routeDomainsByListener(
	route *gatewayv1.Route,
	gatewayID string,
	hostnames []string,
	explicitHostnames bool,
	listenersByGateway map[string][]gatewayListener,
) map[listenerKey]map[string]bool {
	result := make(map[listenerKey]map[string]bool)
	if !explicitHostnames {
		for _, listener := range listenersByGateway[gatewayID] {
			if result[listener.key] == nil {
				result[listener.key] = make(map[string]bool)
			}
			for _, hostname := range listener.hosts {
				result[listener.key][hostname] = true
			}
		}
		return result
	}

	for _, hostname := range hostnames {
		matched := false
		for _, listener := range listenersByGateway[gatewayID] {
			for _, listenerHostname := range listener.hosts {
				if !hostnameCoveredByListener(hostname, listenerHostname) {
					continue
				}
				if result[listener.key] == nil {
					result[listener.key] = make(map[string]bool)
				}
				result[listener.key][hostname] = true
				matched = true
			}
		}
		if !matched {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonConflict, fmt.Sprintf("route %q hostname %q does not belong to a listener on gateway %q", route.Name, hostname, gatewayID))
		}
	}
	return result
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func compareRouteAttachments(a, b routeAttachment) int {
	if result := compareListenerKeys(a.listenerKey, b.listenerKey); result != 0 {
		return result
	}
	if result := cmp.Compare(a.gatewayID, b.gatewayID); result != 0 {
		return result
	}
	return cmp.Compare(a.routeID, b.routeID)
}

func sortedListenerKeySet(values map[listenerKey]map[string]bool) []listenerKey {
	keys := slices.Collect(maps.Keys(values))
	slices.SortFunc(keys, compareListenerKeys)
	return keys
}

func envoyRouteName(gatewayID, routeID, method, variant string) string {
	name := fmt.Sprintf("%s/%s/%s", envoyRouteNamePrefix, gatewayID, routeID)
	if method != "" {
		name += "/" + strings.ToLower(method)
	}
	if variant != "" {
		name += "/" + variant
	}
	return name
}

func virtualHostName(key listenerKey, domain string) string {
	return fmt.Sprintf("%s/%s/%s", virtualHostNamePrefix, listenerName(key), domain)
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
