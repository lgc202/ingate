package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

func (c *compilation) buildRouteMethods(route *gatewayv1.Route) ([]string, bool) {
	if len(route.Spec.Match.Methods) > routeconfig.MaxHTTPMethods {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q declares too many methods", route.Name),
		)
		return nil, false
	}

	methods := make(map[string]bool, len(route.Spec.Match.Methods))
	valid := true
	for _, methodValue := range route.Spec.Match.Methods {
		method := strings.ToUpper(strings.TrimSpace(methodValue))
		if !routeconfig.IsSupportedHTTPMethod(method) || methods[method] {
			c.addRouteError(
				route.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q has invalid or duplicate method %q", route.Name, methodValue),
			)
			valid = false
			continue
		}
		methods[method] = true
	}
	return slices.Sorted(maps.Keys(methods)), valid
}

func (c *compilation) buildHeaderMatchers(route *gatewayv1.Route) ([]*routev3.HeaderMatcher, bool) {
	if len(route.Spec.Match.Headers) > routeconfig.MaxHeaderMatches {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q declares too many header matches", route.Name),
		)
		return nil, false
	}

	headers := slices.Clone(route.Spec.Match.Headers)
	for i := range headers {
		headers[i].Name = httpheader.NormalizeName(headers[i].Name)
		headers[i].Value = httpheader.NormalizeValue(headers[i].Value)
	}
	slices.SortFunc(headers, func(a, b gatewayv1.HeaderMatch) int {
		return cmp.Or(
			cmp.Compare(a.Name, b.Name),
			cmp.Compare(a.Value, b.Value),
		)
	})
	matchers := make([]*routev3.HeaderMatcher, 0, len(headers))
	seenNames := make(map[string]bool, len(headers))
	valid := true
	for _, header := range headers {
		if !httpheader.IsValidName(header.Name) ||
			header.Value == "" ||
			!httpheader.IsValidValue(header.Value) ||
			seenNames[header.Name] {
			c.addRouteError(
				route.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q has an invalid or duplicate header match %q", route.Name, header.Name),
			)
			valid = false
			continue
		}
		seenNames[header.Name] = true
		matchers = append(matchers, exactHeaderMatcher(header.Name, header.Value))
	}
	return matchers, valid
}

func exactHeaderMatcher(name, value string) *routev3.HeaderMatcher {
	return &routev3.HeaderMatcher{
		Name: name,
		HeaderMatchSpecifier: &routev3.HeaderMatcher_StringMatch{
			StringMatch: &matcherv3.StringMatcher{
				MatchPattern: &matcherv3.StringMatcher_Exact{Exact: value},
			},
		},
	}
}

func routeMatchHeaderValues(match *routev3.RouteMatch) map[string]string {
	values := make(map[string]string, len(match.GetHeaders()))
	for _, header := range match.GetHeaders() {
		values[strings.ToLower(header.GetName())] = header.GetStringMatch().GetExact()
	}
	return values
}

func routeHeaderMatchesOverlap(left, right map[string]string) bool {
	if len(left) > len(right) {
		left, right = right, left
	}
	for name, leftValue := range left {
		if rightValue, exists := right[name]; exists && rightValue != leftValue {
			return false
		}
	}
	return true
}
