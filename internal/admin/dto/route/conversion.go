package route

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// Spec 将已校验的 Route 请求转换为声明式 RouteSpec
func (r CreateRouteReq) Spec() resource.RouteSpec {
	return resource.RouteSpec{
		DisplayName: r.Name,
		Enabled:     r.EnabledValue(),
		ParentRefs:  parentRefsFromIDs(r.GatewayIDs),
		Hostnames:   r.Hostnames,
		Rules:       routeRulesFromRequest(r.Rules),
	}
}

func routeRulesFromRequest(requests []RouteRule) []resource.RouteRule {
	rules := make([]resource.RouteRule, 0, len(requests))
	for _, request := range requests {
		rule := resource.RouteRule{
			Name:         request.Name,
			PathPrefix:   request.PathPrefix,
			Methods:      request.Methods,
			Headers:      headerMatchesFromRequest(request.Headers),
			UpstreamRefs: upstreamRefsFromTargets(request.Targets),
			ModelRouting: modelRoutingFromRequest(request.ModelRouting),
		}
		if request.RequestHeaderModifier != nil {
			rule.Filters = append(rule.Filters, resource.RouteFilter{
				Type:                  resource.RouteFilterRequestHeaderModifier,
				RequestHeaderModifier: headerModifierFromRequest(request.RequestHeaderModifier),
			})
		}
		if request.ResponseHeaderModifier != nil {
			rule.Filters = append(rule.Filters, resource.RouteFilter{
				Type:                   resource.RouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: headerModifierFromRequest(request.ResponseHeaderModifier),
			})
		}
		if request.Timeout != nil {
			rule.Timeout = &resource.RouteTimeout{RequestMillis: request.Timeout.RequestMillis}
		}
		if request.Retry != nil {
			rule.Retry = &resource.RouteRetry{
				Attempts:            request.Retry.Attempts,
				PerTryTimeoutMillis: request.Retry.PerTryTimeoutMillis,
			}
		}
		rules = append(rules, rule)
	}
	return rules
}

func modelRoutingFromRequest(request *ModelRouting) *resource.ModelRouting {
	if request == nil {
		return nil
	}
	models := make([]resource.ModelRoute, 0, len(request.Models))
	for _, model := range request.Models {
		models = append(models, resource.ModelRoute{
			Model:         model.Model,
			UpstreamRef:   model.UpstreamID,
			UpstreamModel: model.UpstreamModel,
		})
	}
	return &resource.ModelRouting{Models: models}
}

func parentRefsFromIDs(gatewayIDs []string) []resource.ParentRef {
	refs := make([]resource.ParentRef, 0, len(gatewayIDs))
	for _, gatewayID := range gatewayIDs {
		refs = append(refs, resource.ParentRef{Name: gatewayID})
	}
	return refs
}

func upstreamRefsFromTargets(targets []RouteTarget) []resource.UpstreamRef {
	refs := make([]resource.UpstreamRef, 0, len(targets))
	for _, target := range targets {
		refs = append(refs, resource.UpstreamRef{
			Name:   target.UpstreamID,
			Weight: target.Weight,
		})
	}
	return refs
}

func headerMatchesFromRequest(requests []HeaderMatchReq) []resource.HeaderMatch {
	headers := make([]resource.HeaderMatch, 0, len(requests))
	for _, request := range requests {
		headers = append(headers, resource.HeaderMatch{
			Name:  request.Name,
			Value: request.Value,
		})
	}
	return headers
}

func headerModifierFromRequest(request *HeaderModifierReq) *resource.HeaderModifier {
	headers := make([]resource.HeaderValue, 0, len(request.Set))
	for _, header := range request.Set {
		headers = append(headers, resource.HeaderValue{
			Name:  header.Name,
			Value: header.Value,
		})
	}
	return &resource.HeaderModifier{
		Set:    headers,
		Remove: request.Remove,
	}
}
