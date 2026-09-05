package compiler

import (
	"cmp"
	"fmt"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// routeEntryTemplate 保存普通 Route 和 AI Route 生成 Envoy Route 时共享的匹配与 Header 配置。
type routeEntryTemplate struct {
	path                    string
	exactPath               bool
	methods                 []string
	headers                 []*routev3.HeaderMatcher
	requestHeadersToAdd     []*corev3.HeaderValueOption
	requestHeadersToRemove  []string
	responseHeadersToAdd    []*corev3.HeaderValueOption
	responseHeadersToRemove []string
}

// routeEntry 保存 Envoy Route 及其稳定的匹配优先级元数据。
type routeEntry struct {
	routeID      string
	variant      string
	path         string
	exactPath    bool
	method       string
	headerCount  int
	matchHeaders map[string]string
	route        *routev3.Route
}

// routeMatchClass 保存判断两条 Route 是否处于同一匹配优先级所需的字段。
// method 相同时请求集合才可能相交；routeID 和 variant 只负责稳定排序。
type routeMatchClass struct {
	path        string
	exactPath   bool
	method      string
	headerCount int
}

func (c *compilation) buildRouteEntries(
	route *gatewayv1.Route,
	compiledUpstreams map[string]bool,
) []routeEntry {
	if route.Spec.AI != nil {
		return c.buildAIRouteEntries(route, compiledUpstreams)
	}

	template, templateValid := c.buildRouteEntryTemplate(route)
	clusters, clustersValid := c.buildWeightedClusters(route, compiledUpstreams)
	if !templateValid || !clustersValid {
		return nil
	}

	action, ok := c.buildRouteAction(route, clusters)
	if !ok {
		return nil
	}

	methods := template.methods
	if len(methods) == 0 {
		methods = []string{""}
	}
	entries := make([]routeEntry, 0, len(methods))
	for _, method := range methods {
		match := template.match(method)
		entries = append(entries, routeEntry{
			routeID:      route.Name,
			path:         template.path,
			exactPath:    template.exactPath,
			method:       method,
			headerCount:  len(template.headers),
			matchHeaders: routeMatchHeaderValues(match),
			route: &routev3.Route{
				Match:                   match,
				Action:                  &routev3.Route_Route{Route: action},
				RequestHeadersToAdd:     template.requestHeadersToAdd,
				RequestHeadersToRemove:  template.requestHeadersToRemove,
				ResponseHeadersToAdd:    template.responseHeadersToAdd,
				ResponseHeadersToRemove: template.responseHeadersToRemove,
			},
		})
	}
	return entries
}

func (c *compilation) buildRouteEntryTemplate(route *gatewayv1.Route) (routeEntryTemplate, bool) {
	path := strings.TrimSpace(route.Spec.Match.Path.Value)
	valid := true
	if !routeconfig.IsValidPath(path) {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q has an invalid request path", route.Name),
		)
		valid = false
	}

	exactPath := false
	switch route.Spec.Match.Path.Type {
	case gatewayv1.PathMatchPrefix:
	case gatewayv1.PathMatchExact:
		exactPath = true
	default:
		c.addRouteError(
			route.Name,
			ReasonUnsupported,
			fmt.Sprintf(
				"route %q uses unsupported path match type %q",
				route.Name,
				route.Spec.Match.Path.Type,
			),
		)
		valid = false
	}

	methods, methodsValid := c.buildRouteMethods(route)
	headers, headersValid := c.buildHeaderMatchers(route)
	requestAdd, requestRemove, requestValid := c.buildHeaderModifier(
		route,
		route.Spec.RequestHeaderModifier,
	)
	responseAdd, responseRemove, responseValid := c.buildHeaderModifier(
		route,
		route.Spec.ResponseHeaderModifier,
	)
	return routeEntryTemplate{
		path:                    path,
		exactPath:               exactPath,
		methods:                 methods,
		headers:                 headers,
		requestHeadersToAdd:     requestAdd,
		requestHeadersToRemove:  requestRemove,
		responseHeadersToAdd:    responseAdd,
		responseHeadersToRemove: responseRemove,
	}, valid && methodsValid && headersValid && requestValid && responseValid
}

func (t routeEntryTemplate) match(
	method string,
	additionalHeaders ...*routev3.HeaderMatcher,
) *routev3.RouteMatch {
	headerCount := len(t.headers) + len(additionalHeaders)
	if method != "" {
		headerCount++
	}
	headers := make([]*routev3.HeaderMatcher, 0, headerCount)
	if method != "" {
		headers = append(headers, exactHeaderMatcher(":method", method))
	}
	headers = append(headers, t.headers...)
	headers = append(headers, additionalHeaders...)

	match := &routev3.RouteMatch{Headers: headers}
	if t.exactPath {
		match.PathSpecifier = &routev3.RouteMatch_Path{Path: t.path}
	} else {
		match.PathSpecifier = &routev3.RouteMatch_Prefix{Prefix: t.path}
	}
	return match
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
	return cmp.Or(
		cmp.Compare(a.routeID, b.routeID),
		cmp.Compare(a.method, b.method),
		cmp.Compare(a.variant, b.variant),
	)
}
