package route

import (
	"strconv"
	"time"

	admindto "github.com/lgc202/ingate/internal/admin/dto"
	"github.com/lgc202/ingate/internal/admin/service/resourcestatus"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewListRoutesResp 转换 Route 资源列表为控制台响应
func NewListRoutesResp(resources []resource.Route) ListRoutesResp {
	routes := make([]Route, 0, len(resources))
	for i := range resources {
		routes = append(routes, routeFromResource(&resources[i]))
	}
	return ListRoutesResp{Routes: routes}
}

// NewGetRouteResp 转换 Route 资源为控制台响应
func NewGetRouteResp(route *resource.Route) *Route {
	item := routeFromResource(route)
	return &item
}

func routeFromResource(route *resource.Route) Route {
	status := resourcestatus.FromConditions(route.Generation, route.Status.Conditions)
	if !route.Spec.Enabled && resourcestatus.ConfigurationApplied(route.Generation, route.Status.Conditions) {
		status = resourcestatus.Disabled()
	}
	return Route{
		ID:         route.Name,
		Version:    strconv.FormatInt(route.Generation, 10),
		Status:     admindto.NewResourceStatus(status),
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
			ModelRouting:           modelRouting(rule.ModelRouting),
			RequestHeaderModifier:  requestHeaderModifier(rule),
			ResponseHeaderModifier: responseHeaderModifier(rule),
			Timeout:                routeTimeout(rule.Timeout),
			Retry:                  routeRetry(rule.Retry),
		}
	})
}

func modelRouting(source *resource.ModelRouting) *ModelRouting {
	if source == nil {
		return nil
	}
	return &ModelRouting{
		Models: lo.Map(source.Models, func(model resource.ModelRoute, _ int) ModelRoute {
			return ModelRoute{
				Model:         model.Model,
				UpstreamID:    model.UpstreamRef,
				UpstreamModel: model.UpstreamModel,
			}
		}),
	}
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
