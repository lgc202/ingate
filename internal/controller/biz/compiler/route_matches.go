package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

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

func validRouteMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
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
