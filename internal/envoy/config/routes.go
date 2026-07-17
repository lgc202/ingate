package config

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
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
	runtimeRouteNamePrefix = "ingate-route"
	virtualHostNamePrefix  = "ingate-vhost"
)

type routeAttachment struct {
	listenerKey listenerKey
	gatewayID   string
	routeID     string
	ruleName    string
}

type routeEntry struct {
	gatewayID   string
	routeID     string
	ruleName    string
	pathPrefix  string
	method      string
	methodCount int
	headerCount int
	route       *routev3.Route
}

func (c *compileContext) buildRoutes() []*routev3.RouteConfiguration {
	// Route 会按 ParentRef、Listener 和有效 Host 展开，最终直接形成 Envoy VirtualHost/Route
	// 这里不保留可跨包消费的中间表示，策略索引只记录运行时 route name 中的资源身份
	routesByListener := make(map[listenerKey]map[string][]routeEntry, len(c.listenerGroups))
	matchOwners := make(map[listenerKey]map[string]map[string]routeEntry, len(c.listenerGroups))
	attachmentSet := make(map[string]bool)

	for _, routeID := range slices.Sorted(maps.Keys(c.routes)) {
		route := c.routes[routeID]
		if !route.Spec.Enabled {
			continue
		}
		hostnames, explicitHostnames := c.routeHostnames(route)
		if len(route.Spec.Rules) == 0 {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q must declare at least one rule", routeID),
			)
		}

		parents := make(map[string]bool, len(route.Spec.ParentRefs))
		for _, parentRef := range route.Spec.ParentRefs {
			if parentRef.Name == "" {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindRoute,
					routeID,
					ReasonInvalidSpec,
					fmt.Sprintf("route %q has an empty parent reference", routeID),
				)
				continue
			}
			if parents[parentRef.Name] {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindRoute,
					routeID,
					ReasonConflict,
					fmt.Sprintf("route %q references gateway %q more than once", routeID, parentRef.Name),
				)
				continue
			}
			parents[parentRef.Name] = true
		}
		if len(parents) == 0 {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonConflict,
				fmt.Sprintf("route %q has no attachable gateway", routeID),
			)
			continue
		}

		attachedToGateway := false
		for _, gatewayID := range slices.Sorted(maps.Keys(parents)) {
			gateway, exists := c.gateways[gatewayID]
			if !exists {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindRoute,
					routeID,
					ReasonReferenceNotFound,
					fmt.Sprintf("route %q references missing gateway %q", routeID, gatewayID),
				)
				continue
			}
			if !gateway.Spec.Enabled {
				continue
			}

			domainsByListener := c.routeDomainsByListener(route, gatewayID, hostnames, explicitHostnames)
			if len(domainsByListener) == 0 {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindRoute,
					routeID,
					ReasonConflict,
					fmt.Sprintf("route %q has no attachable listener on gateway %q", routeID, gatewayID),
				)
				continue
			}
			attachedToGateway = true

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
					for _, rule := range route.Spec.Rules {
						entries := c.buildRouteEntries(gatewayID, routeID, rule)
						for _, entry := range entries {
							matchKey := routeMatchKey(entry.route.GetMatch())
							if previous, conflict := matchOwners[key][domain][matchKey]; conflict {
								if previous.route.GetName() == entry.route.GetName() {
									continue
								}
								message := fmt.Sprintf(
									"listener %s hostname %q has the same route match in %q/%q and %q/%q",
									listenerName(key),
									domain,
									previous.routeID,
									previous.ruleName,
									entry.routeID,
									entry.ruleName,
								)
								c.addDiagnostic(SeverityError, gatewayv1.KindRoute, previous.routeID, ReasonConflict, message)
								c.addDiagnostic(SeverityError, gatewayv1.KindRoute, entry.routeID, ReasonConflict, message)
								continue
							}
							matchOwners[key][domain][matchKey] = entry
							routesByListener[key][domain] = append(routesByListener[key][domain], entry)

							attachmentKey := fmt.Sprintf("%s\x00%s\x00%s\x00%s", listenerName(key), gatewayID, routeID, rule.Name)
							if !attachmentSet[attachmentKey] {
								attachmentSet[attachmentKey] = true
								c.routeAttachments = append(c.routeAttachments, routeAttachment{
									listenerKey: key,
									gatewayID:   gatewayID,
									routeID:     routeID,
									ruleName:    rule.Name,
								})
							}
						}
					}
				}
			}
		}
		if !attachedToGateway {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonConflict,
				fmt.Sprintf("route %q has no attachable listener", routeID),
			)
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
		configs = append(configs, &routev3.RouteConfiguration{
			Name:         routeConfigName(key),
			VirtualHosts: virtualHosts,
		})
	}
	return configs
}

func (c *compileContext) routeHostnames(route *gatewayv1.Route) ([]string, bool) {
	if len(route.Spec.Hostnames) == 0 {
		return nil, false
	}

	hostnames := make(map[string]bool, len(route.Spec.Hostnames))
	for _, value := range route.Spec.Hostnames {
		hostname, ok := normalizeHostname(value)
		if !ok || hostname == "*" {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				route.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q has invalid hostname %q", route.Name, value),
			)
			continue
		}
		if hostnames[hostname] {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				route.Name,
				ReasonConflict,
				fmt.Sprintf("route %q declares hostname %q more than once", route.Name, hostname),
			)
			continue
		}
		hostnames[hostname] = true
	}
	return slices.Sorted(maps.Keys(hostnames)), true
}

func (c *compileContext) routeDomainsByListener(
	route *gatewayv1.Route,
	gatewayID string,
	hostnames []string,
	explicitHostnames bool,
) map[listenerKey]map[string]bool {
	result := make(map[listenerKey]map[string]bool)
	if !explicitHostnames {
		// Route 未声明 Hostname 时继承 Listener 的 Host 所有权，而不是扩大成全局 catch-all
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
				break
			}
		}
		if !matched {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				route.Name,
				ReasonConflict,
				fmt.Sprintf("route %q hostname %q does not belong to a listener on gateway %q", route.Name, hostname, gatewayID),
			)
		}
	}
	return result
}

func (c *compileContext) buildRouteEntries(
	gatewayID string,
	routeID string,
	rule gatewayv1.RouteRule,
) []routeEntry {
	if rule.Name == "" {
		return nil
	}
	pathPrefix := rule.PathPrefix
	if pathPrefix == "" || !strings.HasPrefix(pathPrefix, "/") {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindRoute,
			routeID,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q rule %q pathPrefix must start with /", routeID, rule.Name),
		)
		return nil
	}

	methods, methodsValid := c.routeMethods(routeID, rule)
	clusters, clustersValid := c.weightedClusters(routeID, rule)
	requestHeadersToAdd, requestHeadersToRemove, responseHeadersToAdd, responseHeadersToRemove, filtersValid := c.routeFilters(routeID, rule)
	headers, headersValid := c.routeHeaderMatches(routeID, rule)
	if !methodsValid || !clustersValid || !filtersValid || !headersValid {
		return nil
	}

	action := &routev3.RouteAction{
		ClusterSpecifier: &routev3.RouteAction_WeightedClusters{
			WeightedClusters: &routev3.WeightedCluster{Clusters: clusters},
		},
	}
	if rule.Timeout != nil {
		if rule.Timeout.RequestMillis < minRouteTimeoutMillis ||
			rule.Timeout.RequestMillis > maxRouteTimeoutMillis {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonInvalidSpec,
				fmt.Sprintf(
					"route %q rule %q timeout must be between %d and %d milliseconds",
					routeID,
					rule.Name,
					minRouteTimeoutMillis,
					maxRouteTimeoutMillis,
				),
			)
			return nil
		}
		action.Timeout = durationpb.New(time.Duration(rule.Timeout.RequestMillis) * time.Millisecond)
	}
	if rule.Retry != nil {
		retryPolicy, ok := c.routeRetryPolicy(routeID, rule)
		if !ok {
			return nil
		}
		action.RetryPolicy = retryPolicy
	}

	methodValues := methods
	if len(methodValues) == 0 {
		methodValues = []string{""}
	}
	entries := make([]routeEntry, 0, len(methodValues))
	for _, method := range methodValues {
		routeHeaders := slices.Clone(headers)
		if method != "" {
			routeHeaders = append([]*routev3.HeaderMatcher{
				{
					Name: ":method",
					HeaderMatchSpecifier: &routev3.HeaderMatcher_ExactMatch{
						ExactMatch: method,
					},
				},
			}, routeHeaders...)
		}
		entries = append(entries, routeEntry{
			gatewayID:   gatewayID,
			routeID:     routeID,
			ruleName:    rule.Name,
			pathPrefix:  pathPrefix,
			method:      method,
			methodCount: len(methods),
			headerCount: len(headers),
			route: &routev3.Route{
				Name: runtimeRouteName(gatewayID, routeID, rule.Name, method),
				Match: &routev3.RouteMatch{
					PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: pathPrefix},
					Headers:       routeHeaders,
				},
				Action:                  &routev3.Route_Route{Route: action},
				RequestHeadersToAdd:     requestHeadersToAdd,
				RequestHeadersToRemove:  requestHeadersToRemove,
				ResponseHeadersToAdd:    responseHeadersToAdd,
				ResponseHeadersToRemove: responseHeadersToRemove,
			},
		})
	}
	return entries
}

func (c *compileContext) routeMethods(routeID string, rule gatewayv1.RouteRule) ([]string, bool) {
	methods := make(map[string]bool, len(rule.Methods))
	valid := true
	for _, value := range rule.Methods {
		method := strings.ToUpper(strings.TrimSpace(value))
		if !validRouteMethod(method) {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q rule %q has invalid method %q", routeID, rule.Name, value),
			)
			valid = false
			continue
		}
		if methods[method] {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonConflict,
				fmt.Sprintf("route %q rule %q declares method %q more than once", routeID, rule.Name, method),
			)
			valid = false
			continue
		}
		methods[method] = true
	}
	return slices.Sorted(maps.Keys(methods)), valid
}

func (c *compileContext) routeHeaderMatches(routeID string, rule gatewayv1.RouteRule) ([]*routev3.HeaderMatcher, bool) {
	items := slices.Clone(rule.Headers)
	slices.SortFunc(items, func(a, b gatewayv1.HeaderMatch) int {
		aName := strings.ToLower(a.Name)
		bName := strings.ToLower(b.Name)
		if aName != bName {
			return cmp.Compare(aName, bName)
		}
		return cmp.Compare(a.Value, b.Value)
	})

	result := make([]*routev3.HeaderMatcher, 0, len(items))
	seen := make(map[string]bool, len(items))
	valid := true
	for _, header := range items {
		if header.Name == "" {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q rule %q has a header match without a name", routeID, rule.Name),
			)
			valid = false
			continue
		}
		if header.Value == "" {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q rule %q header match %q has an empty value", routeID, rule.Name, header.Name),
			)
			valid = false
			continue
		}
		key := strings.ToLower(header.Name) + "\x00" + header.Value
		if seen[key] {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonConflict,
				fmt.Sprintf("route %q rule %q repeats header match %q", routeID, rule.Name, header.Name),
			)
			valid = false
			continue
		}
		seen[key] = true
		result = append(result, &routev3.HeaderMatcher{
			Name: header.Name,
			HeaderMatchSpecifier: &routev3.HeaderMatcher_ExactMatch{
				ExactMatch: header.Value,
			},
		})
	}
	return result, valid
}

func (c *compileContext) weightedClusters(
	routeID string,
	rule gatewayv1.RouteRule,
) ([]*routev3.WeightedCluster_ClusterWeight, bool) {
	if len(rule.UpstreamRefs) == 0 {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindRoute,
			routeID,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q rule %q must reference at least one upstream", routeID, rule.Name),
		)
		return nil, false
	}

	refs := slices.Clone(rule.UpstreamRefs)
	slices.SortFunc(refs, func(a, b gatewayv1.UpstreamRef) int {
		return cmp.Compare(a.Name, b.Name)
	})
	clusters := make([]*routev3.WeightedCluster_ClusterWeight, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	valid := true
	for _, ref := range refs {
		if ref.Name == "" {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q rule %q has an empty upstream reference", routeID, rule.Name),
			)
			valid = false
			continue
		}
		if seen[ref.Name] {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonConflict,
				fmt.Sprintf("route %q rule %q references upstream %q more than once", routeID, rule.Name, ref.Name),
			)
			valid = false
			continue
		}
		seen[ref.Name] = true
		if _, exists := c.upstreams[ref.Name]; !exists {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonReferenceNotFound,
				fmt.Sprintf("route %q rule %q references missing upstream %q", routeID, rule.Name, ref.Name),
			)
			valid = false
			continue
		}
		if ref.Weight < 1 || ref.Weight > 1000 {
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q rule %q upstream %q weight must be between 1 and 1000", routeID, rule.Name, ref.Name),
			)
			valid = false
			continue
		}
		clusters = append(clusters, &routev3.WeightedCluster_ClusterWeight{
			Name:   ref.Name,
			Weight: wrapperspb.UInt32(uint32(ref.Weight)),
		})
	}
	return clusters, valid
}

func (c *compileContext) routeFilters(
	routeID string,
	rule gatewayv1.RouteRule,
) (
	[]*corev3.HeaderValueOption,
	[]string,
	[]*corev3.HeaderValueOption,
	[]string,
	bool,
) {
	var requestHeadersToAdd []*corev3.HeaderValueOption
	var requestHeadersToRemove []string
	var responseHeadersToAdd []*corev3.HeaderValueOption
	var responseHeadersToRemove []string
	valid := true

	for _, filter := range rule.Filters {
		switch filter.Type {
		case gatewayv1.RouteFilterRequestHeaderModifier:
			if filter.RequestHeaderModifier == nil {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindRoute,
					routeID,
					ReasonInvalidSpec,
					fmt.Sprintf("route %q rule %q request header modifier is empty", routeID, rule.Name),
				)
				valid = false
				continue
			}
			values, remove, ok := c.headerModifier(routeID, rule.Name, filter.RequestHeaderModifier)
			requestHeadersToAdd = append(requestHeadersToAdd, values...)
			requestHeadersToRemove = append(requestHeadersToRemove, remove...)
			valid = valid && ok
		case gatewayv1.RouteFilterResponseHeaderModifier:
			if filter.ResponseHeaderModifier == nil {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindRoute,
					routeID,
					ReasonInvalidSpec,
					fmt.Sprintf("route %q rule %q response header modifier is empty", routeID, rule.Name),
				)
				valid = false
				continue
			}
			values, remove, ok := c.headerModifier(routeID, rule.Name, filter.ResponseHeaderModifier)
			responseHeadersToAdd = append(responseHeadersToAdd, values...)
			responseHeadersToRemove = append(responseHeadersToRemove, remove...)
			valid = valid && ok
		default:
			c.addDiagnostic(
				SeverityError,
				gatewayv1.KindRoute,
				routeID,
				ReasonUnsupported,
				fmt.Sprintf("route %q rule %q uses unsupported filter %q", routeID, rule.Name, filter.Type),
			)
			valid = false
		}
	}
	return requestHeadersToAdd, requestHeadersToRemove, responseHeadersToAdd, responseHeadersToRemove, valid
}

func (c *compileContext) headerModifier(
	routeID string,
	ruleName string,
	modifier *gatewayv1.HeaderModifier,
) ([]*corev3.HeaderValueOption, []string, bool) {
	values := make([]*corev3.HeaderValueOption, 0, len(modifier.Set)+len(modifier.Add))
	valid := true
	if len(modifier.Set) == 0 && len(modifier.Add) == 0 && len(modifier.Remove) == 0 {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindRoute,
			routeID,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q rule %q header modifier has no actions", routeID, ruleName),
		)
		valid = false
	}
	for _, header := range modifier.Set {
		if header.Name == "" || header.Value == "" {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q has an invalid Set header", routeID, ruleName))
			valid = false
			continue
		}
		values = append(values, headerValueOption(header, corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD))
	}
	for _, header := range modifier.Add {
		if header.Name == "" || header.Value == "" {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q has an invalid Add header", routeID, ruleName))
			valid = false
			continue
		}
		values = append(values, headerValueOption(header, corev3.HeaderValueOption_APPEND_IF_EXISTS_OR_ADD))
	}
	remove := slices.Clone(modifier.Remove)
	for _, name := range remove {
		if name != "" {
			continue
		}
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, routeID, ReasonInvalidSpec, fmt.Sprintf("route %q rule %q has an empty Remove header name", routeID, ruleName))
		valid = false
	}
	return values, remove, valid
}

func (c *compileContext) routeRetryPolicy(routeID string, rule gatewayv1.RouteRule) (*routev3.RetryPolicy, bool) {
	if rule.Retry.Attempts < minRetryAttempts || rule.Retry.Attempts > maxRetryAttempts {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindRoute,
			routeID,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"route %q rule %q retry attempts must be between %d and %d",
				routeID,
				rule.Name,
				minRetryAttempts,
				maxRetryAttempts,
			),
		)
		return nil, false
	}
	if rule.Retry.PerTryTimeoutMillis < minPerTryTimeoutMillis ||
		rule.Retry.PerTryTimeoutMillis > maxPerTryTimeoutMillis {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindRoute,
			routeID,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"route %q rule %q retry per-try timeout must be between %d and %d milliseconds",
				routeID,
				rule.Name,
				minPerTryTimeoutMillis,
				maxPerTryTimeoutMillis,
			),
		)
		return nil, false
	}
	if rule.Timeout != nil && rule.Retry.PerTryTimeoutMillis > rule.Timeout.RequestMillis {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindRoute,
			routeID,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q rule %q retry per-try timeout exceeds request timeout", routeID, rule.Name),
		)
		return nil, false
	}

	retryOn := defaultRetryOn
	if len(rule.Retry.RetryOn) > 0 {
		values := make(map[string]bool, len(rule.Retry.RetryOn))
		for _, value := range rule.Retry.RetryOn {
			value = strings.TrimSpace(value)
			if value == "" || strings.Contains(value, ",") {
				c.addDiagnostic(
					SeverityError,
					gatewayv1.KindRoute,
					routeID,
					ReasonInvalidSpec,
					fmt.Sprintf("route %q rule %q has invalid retryOn value %q", routeID, rule.Name, value),
				)
				return nil, false
			}
			values[value] = true
		}
		retryOn = strings.Join(slices.Sorted(maps.Keys(values)), ",")
	}

	policy := &routev3.RetryPolicy{
		RetryOn:    retryOn,
		NumRetries: wrapperspb.UInt32(uint32(rule.Retry.Attempts)),
	}
	if rule.Retry.PerTryTimeoutMillis > 0 {
		policy.PerTryTimeout = durationpb.New(time.Duration(rule.Retry.PerTryTimeoutMillis) * time.Millisecond)
	}
	return policy, true
}

func validRouteMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func headerValueOption(value gatewayv1.HeaderValue, action corev3.HeaderValueOption_HeaderAppendAction) *corev3.HeaderValueOption {
	return &corev3.HeaderValueOption{
		Header: &corev3.HeaderValue{
			Key:   value.Name,
			Value: value.Value,
		},
		AppendAction: action,
	}
}

func routeMatchKey(match *routev3.RouteMatch) string {
	parts := []string{match.GetPrefix()}
	for _, header := range match.GetHeaders() {
		parts = append(parts, strings.ToLower(header.GetName())+"="+header.GetExactMatch())
	}
	return strings.Join(parts, "\x00")
}

func compareRouteEntries(a, b routeEntry) int {
	if len(a.pathPrefix) != len(b.pathPrefix) {
		return cmp.Compare(len(b.pathPrefix), len(a.pathPrefix))
	}
	if a.methodCount != b.methodCount {
		return cmp.Compare(b.methodCount, a.methodCount)
	}
	if a.headerCount != b.headerCount {
		return cmp.Compare(b.headerCount, a.headerCount)
	}
	if a.routeID != b.routeID {
		return cmp.Compare(a.routeID, b.routeID)
	}
	if a.ruleName != b.ruleName {
		return cmp.Compare(a.ruleName, b.ruleName)
	}
	return cmp.Compare(a.method, b.method)
}

func compareRouteAttachments(a, b routeAttachment) int {
	if result := compareListenerKeys(a.listenerKey, b.listenerKey); result != 0 {
		return result
	}
	if a.gatewayID != b.gatewayID {
		return cmp.Compare(a.gatewayID, b.gatewayID)
	}
	if a.routeID != b.routeID {
		return cmp.Compare(a.routeID, b.routeID)
	}
	return cmp.Compare(a.ruleName, b.ruleName)
}

func sortedListenerKeySet(values map[listenerKey]map[string]bool) []listenerKey {
	keys := slices.Collect(maps.Keys(values))
	slices.SortFunc(keys, compareListenerKeys)
	return keys
}

func runtimeRouteName(gatewayID, routeID, ruleName, method string) string {
	return fmt.Sprintf(
		"%s/%s/%s/%s/%s",
		runtimeRouteNamePrefix,
		url.PathEscape(gatewayID),
		url.PathEscape(routeID),
		url.PathEscape(ruleName),
		url.PathEscape(method),
	)
}

func virtualHostName(key listenerKey, domain string) string {
	return fmt.Sprintf("%s/%s/%s", virtualHostNamePrefix, url.PathEscape(listenerName(key)), url.PathEscape(domain))
}
