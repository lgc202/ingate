package compiler

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// routeEntry 保存 Envoy Route 及其稳定的匹配优先级元数据
type routeEntry struct {
	routeID     string
	variant     string
	path        string
	exactPath   bool
	method      string
	headerCount int
	route       *routev3.Route
}

func (c *compilation) buildRouteEntries(route *gatewayv1.Route, compiledUpstreams map[string]bool) []routeEntry {
	if route.Spec.AI != nil {
		return c.buildAIRouteEntries(route, compiledUpstreams)
	}

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
	clusters, clustersValid := c.weightedClusters(route, compiledUpstreams)
	requestAdd, requestRemove, requestValid := c.headerModifier(route, route.Spec.RequestHeaderModifier)
	responseAdd, responseRemove, responseValid := c.headerModifier(route, route.Spec.ResponseHeaderModifier)
	if !methodsValid || !headersValid || !clustersValid || !requestValid || !responseValid {
		return nil
	}

	action, ok := c.routeAction(route, clusters)
	if !ok {
		return nil
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
			routeID:     route.Name,
			path:        path,
			exactPath:   exactPath,
			method:      method,
			headerCount: len(headers),
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
	if result := cmp.Compare(a.method, b.method); result != 0 {
		return result
	}
	return cmp.Compare(a.variant, b.variant)
}
