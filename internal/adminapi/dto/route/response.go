package route

import (
	"time"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewListRoutesResp 转换 Route 列表用例结果为控制台响应
func NewListRoutesResp(result *routeservice.ListResult) ListRoutesResp {
	routes := make([]Route, 0, len(result.Routes))
	for i := range result.Routes {
		routes = append(routes, routeFromResource(&result.Routes[i]))
	}
	return ListRoutesResp{Routes: routes}
}

// NewGetRouteResp 转换单个 Route 用例结果为控制台响应
func NewGetRouteResp(result *routeservice.RouteResult) *Route {
	item := routeFromResource(result.Route)
	return &item
}

func routeFromResource(route *resource.Route) Route {
	status := admindto.NewResourceStatus(route.Generation, route.Status.Conditions)
	if !route.Spec.Enabled && admindto.ConfigurationApplied(route.Generation, route.Status.Conditions) {
		status = admindto.NewDisabledResourceStatus()
	}
	return Route{
		ID:         route.Name,
		Version:    route.ResourceVersion,
		Status:     status,
		Name:       route.Spec.DisplayName,
		GatewayIDs: parentRefIDs(route.Spec.ParentRefs),
		Hostnames:  route.Spec.Hostnames,
		Rules:      routeRules(route.Spec.Rules),
		Enabled:    route.Spec.Enabled,
		CreatedAt:  createdAt(route.ObjectMeta),
	}
}

func routeRules(rules []resource.RouteRule) []RouteRule {
	return lo.Map(rules, func(rule resource.RouteRule, _ int) RouteRule {
		return RouteRule{
			Name:                   rule.Name,
			PathPrefix:             rule.PathPrefix,
			Methods:                rule.Methods,
			Headers:                headerMatches(rule.Headers),
			Targets:                targetServices(rule),
			RequestHeaderModifier:  requestHeaderModifier(rule),
			ResponseHeaderModifier: responseHeaderModifier(rule),
			Timeout:                routeTimeout(rule.Timeout),
			Retry:                  routeRetry(rule.Retry),
		}
	})
}

func parentRefIDs(parentRefs []resource.ParentRef) []string {
	return lo.Map(parentRefs, func(parentRef resource.ParentRef, _ int) string {
		return parentRef.Name
	})
}

func targetServices(rule resource.RouteRule) []RouteTarget {
	return lo.Map(rule.UpstreamRefs, func(ref resource.UpstreamRef, _ int) RouteTarget {
		return RouteTarget{
			UpstreamID: ref.Name,
			Weight:     ref.Weight,
		}
	})
}

func headerMatches(headers []resource.HeaderMatch) []HeaderMatchReq {
	return lo.Map(headers, func(header resource.HeaderMatch, _ int) HeaderMatchReq {
		return HeaderMatchReq{
			Name:  header.Name,
			Value: header.Value,
		}
	})
}

func requestHeaderModifier(rule resource.RouteRule) *HeaderModifierReq {
	for _, filter := range rule.Filters {
		if filter.Type != resource.RouteFilterRequestHeaderModifier || filter.RequestHeaderModifier == nil {
			continue
		}

		return headerModifier(filter.RequestHeaderModifier)
	}
	return nil
}

func responseHeaderModifier(rule resource.RouteRule) *HeaderModifierReq {
	for _, filter := range rule.Filters {
		if filter.Type != resource.RouteFilterResponseHeaderModifier || filter.ResponseHeaderModifier == nil {
			continue
		}

		return headerModifier(filter.ResponseHeaderModifier)
	}
	return nil
}

func headerModifier(source *resource.HeaderModifier) *HeaderModifierReq {
	return &HeaderModifierReq{
		Set: lo.Map(source.Set, func(header resource.HeaderValue, _ int) HeaderValueReq {
			return HeaderValueReq{
				Name:  header.Name,
				Value: header.Value,
			}
		}),
		Remove: source.Remove,
	}
}

func routeTimeout(timeout *resource.RouteTimeout) *RouteTimeoutReq {
	if timeout == nil || timeout.RequestMillis <= 0 {
		return nil
	}
	return &RouteTimeoutReq{RequestMillis: timeout.RequestMillis}
}

func routeRetry(retry *resource.RouteRetry) *RouteRetryReq {
	if retry == nil || retry.Attempts <= 0 {
		return nil
	}
	return &RouteRetryReq{
		Attempts:            retry.Attempts,
		PerTryTimeoutMillis: retry.PerTryTimeoutMillis,
	}
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
