package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	minRouteTimeoutMillis  = 100
	maxRouteTimeoutMillis  = 300000
	minRetryAttempts       = 1
	maxRetryAttempts       = 5
	minPerTryTimeoutMillis = 100
	maxPerTryTimeoutMillis = 60000
	defaultRetryOn         = "connect-failure,refused-stream,reset,5xx"
	envoyRouteNamePrefix   = "ingate-route"
	virtualHostNamePrefix  = "ingate-vhost"
)

type routeAttachment struct {
	listenerKey listenerKey
	gatewayID   string
	routeID     string
}

type routeEntry struct {
	routeID     string
	path        string
	exactPath   bool
	method      string
	headerCount int
	route       *routev3.Route
}

func (c *compilation) buildRoutes() []*routev3.RouteConfiguration {
	routesByListener := make(map[listenerKey]map[string][]routeEntry, len(c.listenerGroups))
	matchOwners := make(map[listenerKey]map[string]map[string]routeEntry, len(c.listenerGroups))
	attachmentSet := make(map[string]bool)

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
		entries := c.buildRouteEntries(route)
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
			domainsByListener := c.routeDomainsByListener(route, gatewayID, hostnames, explicitHostnames)
			if len(domainsByListener) == 0 {
				c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonConflict, fmt.Sprintf("route %q has no attachable listener on gateway %q", routeID, gatewayID))
				continue
			}
			attached = true
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
						current.route.Name = envoyRouteName(gatewayID, routeID, entry.method)
						matchKey := routeMatchKey(current.route.Match)
						if previous, conflict := matchOwners[key][domain][matchKey]; conflict {
							message := fmt.Sprintf("listener %s hostname %q has the same route match in %q and %q", listenerName(key), domain, previous.routeID, routeID)
							c.addDiagnostic(SeverityError, gatewayv1.KindRoute, previous.routeID, ReasonConflict, message)
							c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonConflict, message)
							continue
						}
						matchOwners[key][domain][matchKey] = current
						routesByListener[key][domain] = append(routesByListener[key][domain], current)
					}
				}

				attachmentKey := listenerName(key) + "\x00" + gatewayID + "\x00" + routeID
				if !attachmentSet[attachmentKey] {
					attachmentSet[attachmentKey] = true
					c.routeAttachments = append(c.routeAttachments, routeAttachment{listenerKey: key, gatewayID: gatewayID, routeID: routeID})
				}
			}
		}
		if !attached {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonConflict, fmt.Sprintf("route %q has no attachable listener", routeID))
		}
	}

	slices.SortFunc(c.routeAttachments, compareRouteAttachments)
	configs := make([]*routev3.RouteConfiguration, 0, len(c.listenerGroups))
	for _, key := range c.sortedListenerKeys() {
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
	return configs
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

func (c *compilation) routeDomainsByListener(route *gatewayv1.Route, gatewayID string, hostnames []string, explicit bool) map[listenerKey]map[string]bool {
	result := make(map[listenerKey]map[string]bool)
	if !explicit {
		for _, listener := range c.gatewayListeners[gatewayID] {
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
		for _, listener := range c.gatewayListeners[gatewayID] {
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

func (c *compilation) buildRouteEntries(route *gatewayv1.Route) []routeEntry {
	path := strings.TrimSpace(route.Spec.Match.Path.Value)
	if path == "" || !strings.HasPrefix(path, "/") {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q path must start with /", route.Name))
		return nil
	}
	exactPath := route.Spec.Match.Path.Type == gatewayv1.PathMatchExact
	if !exactPath && route.Spec.Match.Path.Type != gatewayv1.PathMatchPrefix {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonUnsupported, fmt.Sprintf("route %q uses unsupported path match type %q", route.Name, route.Spec.Match.Path.Type))
		return nil
	}

	methods, methodsValid := c.routeMethods(route)
	headers, headersValid := c.routeHeaderMatches(route)
	clusters, clustersValid := c.weightedClusters(route)
	requestAdd, requestRemove, requestValid := c.headerModifier(route, route.Spec.RequestHeaderModifier)
	responseAdd, responseRemove, responseValid := c.headerModifier(route, route.Spec.ResponseHeaderModifier)
	if !methodsValid || !headersValid || !clustersValid || !requestValid || !responseValid {
		return nil
	}

	action := &routev3.RouteAction{ClusterSpecifier: &routev3.RouteAction_WeightedClusters{
		WeightedClusters: &routev3.WeightedCluster{Clusters: clusters},
	}}
	if route.Spec.Timeout != nil {
		if route.Spec.Timeout.RequestMillis < minRouteTimeoutMillis || route.Spec.Timeout.RequestMillis > maxRouteTimeoutMillis {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q timeout is out of range", route.Name))
			return nil
		}
		action.Timeout = durationpb.New(time.Duration(route.Spec.Timeout.RequestMillis) * time.Millisecond)
	}
	if route.Spec.Retry != nil {
		retry, ok := c.routeRetryPolicy(route)
		if !ok {
			return nil
		}
		action.RetryPolicy = retry
	}

	if len(methods) == 0 {
		methods = []string{""}
	}
	entries := make([]routeEntry, 0, len(methods))
	for _, method := range methods {
		routeHeaders := slices.Clone(headers)
		if method != "" {
			routeHeaders = append([]*routev3.HeaderMatcher{exactHeaderMatcher(":method", method)}, routeHeaders...)
		}
		match := &routev3.RouteMatch{Headers: routeHeaders}
		if exactPath {
			match.PathSpecifier = &routev3.RouteMatch_Path{Path: path}
		} else {
			match.PathSpecifier = &routev3.RouteMatch_Prefix{Prefix: path}
		}
		entries = append(entries, routeEntry{
			routeID: route.Name, path: path, exactPath: exactPath, method: method, headerCount: len(headers),
			route: &routev3.Route{
				Match:                   match,
				Action:                  &routev3.Route_Route{Route: proto.Clone(action).(*routev3.RouteAction)},
				RequestHeadersToAdd:     requestAdd,
				RequestHeadersToRemove:  requestRemove,
				ResponseHeadersToAdd:    responseAdd,
				ResponseHeadersToRemove: responseRemove,
			},
		})
	}
	return entries
}

func (c *compilation) routeMethods(route *gatewayv1.Route) ([]string, bool) {
	methods := make(map[string]bool, len(route.Spec.Match.Methods))
	valid := true
	for _, value := range route.Spec.Match.Methods {
		method := strings.ToUpper(strings.TrimSpace(value))
		if !validRouteMethod(method) || methods[method] {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q has invalid or duplicate method %q", route.Name, value))
			valid = false
			continue
		}
		methods[method] = true
	}
	return slices.Sorted(maps.Keys(methods)), valid
}

func (c *compilation) routeHeaderMatches(route *gatewayv1.Route) ([]*routev3.HeaderMatcher, bool) {
	items := slices.Clone(route.Spec.Match.Headers)
	slices.SortFunc(items, func(a, b gatewayv1.HeaderMatch) int {
		if result := cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); result != 0 {
			return result
		}
		return cmp.Compare(a.Value, b.Value)
	})
	result := make([]*routev3.HeaderMatcher, 0, len(items))
	seen := make(map[string]bool, len(items))
	valid := true
	for _, header := range items {
		key := strings.ToLower(header.Name)
		if header.Name == "" || header.Value == "" || seen[key] {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q has an invalid or duplicate header match %q", route.Name, header.Name))
			valid = false
			continue
		}
		seen[key] = true
		result = append(result, exactHeaderMatcher(header.Name, header.Value))
	}
	return result, valid
}

func (c *compilation) weightedClusters(route *gatewayv1.Route) ([]*routev3.WeightedCluster_ClusterWeight, bool) {
	if len(route.Spec.UpstreamRefs) == 0 {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q must reference at least one upstream", route.Name))
		return nil, false
	}
	refs := slices.Clone(route.Spec.UpstreamRefs)
	slices.SortFunc(refs, func(a, b gatewayv1.UpstreamRef) int { return cmp.Compare(a.Name, b.Name) })
	clusters := make([]*routev3.WeightedCluster_ClusterWeight, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	valid := true
	for _, ref := range refs {
		clusterName, exists := c.upstreamClusters[ref.Name]
		if ref.Name == "" || seen[ref.Name] || !exists || ref.Weight < 1 || ref.Weight > 1000 {
			reason := ReasonInvalidSpec
			if ref.Name != "" && !exists {
				reason = ReasonReferenceNotFound
			}
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, reason, fmt.Sprintf("route %q has an invalid upstream reference %q", route.Name, ref.Name))
			valid = false
			continue
		}
		seen[ref.Name] = true
		clusters = append(clusters, &routev3.WeightedCluster_ClusterWeight{Name: clusterName, Weight: wrapperspb.UInt32(uint32(ref.Weight))})
	}
	return clusters, valid
}

func (c *compilation) headerModifier(route *gatewayv1.Route, modifier *gatewayv1.HeaderModifier) ([]*corev3.HeaderValueOption, []string, bool) {
	if modifier == nil {
		return nil, nil, true
	}
	values := make([]*corev3.HeaderValueOption, 0, len(modifier.Set)+len(modifier.Add))
	valid := len(modifier.Set)+len(modifier.Add)+len(modifier.Remove) > 0
	for _, header := range modifier.Set {
		if header.Name == "" || header.Value == "" {
			valid = false
			continue
		}
		values = append(values, headerValueOption(header, corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD))
	}
	for _, header := range modifier.Add {
		if header.Name == "" || header.Value == "" {
			valid = false
			continue
		}
		values = append(values, headerValueOption(header, corev3.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD))
	}
	for _, name := range modifier.Remove {
		if name == "" {
			valid = false
		}
	}
	if !valid {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q has an invalid header modifier", route.Name))
	}
	return values, slices.Clone(modifier.Remove), valid
}

func (c *compilation) routeRetryPolicy(route *gatewayv1.Route) (*routev3.RetryPolicy, bool) {
	retry := route.Spec.Retry
	if retry.Attempts < minRetryAttempts || retry.Attempts > maxRetryAttempts ||
		retry.PerTryTimeoutMillis < minPerTryTimeoutMillis || retry.PerTryTimeoutMillis > maxPerTryTimeoutMillis ||
		(route.Spec.Timeout != nil && retry.PerTryTimeoutMillis > route.Spec.Timeout.RequestMillis) {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q has an invalid retry policy", route.Name))
		return nil, false
	}
	return &routev3.RetryPolicy{
		RetryOn:       defaultRetryOn,
		NumRetries:    wrapperspb.UInt32(uint32(retry.Attempts)),
		PerTryTimeout: durationpb.New(time.Duration(retry.PerTryTimeoutMillis) * time.Millisecond),
	}, true
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

func validRouteMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func headerValueOption(value gatewayv1.HeaderValue, action corev3.HeaderValueOption_HeaderAppendAction) *corev3.HeaderValueOption {
	return &corev3.HeaderValueOption{
		Header:       &corev3.HeaderValue{Key: value.Name, Value: value.Value},
		AppendAction: action,
	}
}

func exactHeaderMatcher(name, value string) *routev3.HeaderMatcher {
	return &routev3.HeaderMatcher{
		Name: name,
		HeaderMatchSpecifier: &routev3.HeaderMatcher_StringMatch{
			StringMatch: &matcherv3.StringMatcher{MatchPattern: &matcherv3.StringMatcher_Exact{Exact: value}},
		},
	}
}

func routeMatchKey(match *routev3.RouteMatch) string {
	path := "prefix=" + match.GetPrefix()
	if match.GetPath() != "" {
		path = "exact=" + match.GetPath()
	}
	parts := []string{path}
	for _, header := range match.GetHeaders() {
		parts = append(parts, strings.ToLower(header.GetName())+"="+header.GetStringMatch().GetExact())
	}
	return strings.Join(parts, "\x00")
}

func compareRouteEntries(a, b routeEntry) int {
	if len(a.path) != len(b.path) {
		return cmp.Compare(len(b.path), len(a.path))
	}
	if a.exactPath != b.exactPath {
		if a.exactPath {
			return -1
		}
		return 1
	}
	if a.headerCount != b.headerCount {
		return cmp.Compare(b.headerCount, a.headerCount)
	}
	if (a.method == "") != (b.method == "") {
		if a.method != "" {
			return -1
		}
		return 1
	}
	if result := cmp.Compare(a.routeID, b.routeID); result != 0 {
		return result
	}
	return cmp.Compare(a.method, b.method)
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

func envoyRouteName(gatewayID, routeID, method string) string {
	name := fmt.Sprintf("%s/%s/%s", envoyRouteNamePrefix, gatewayID, routeID)
	if method != "" {
		name += "/" + strings.ToLower(method)
	}
	return name
}

func virtualHostName(key listenerKey, domain string) string {
	return fmt.Sprintf("%s/%s/%s", virtualHostNamePrefix, listenerName(key), domain)
}
