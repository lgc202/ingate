package route

import (
	"github.com/samber/lo"

	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
)

// Params 将已校验的创建请求转换为 service 参数
func (r CreateRouteReq) Params() routeservice.CreateRouteParams {
	return routeservice.CreateRouteParams{
		Name:       r.Name,
		GatewayIDs: r.GatewayIDs,
		Hostnames:  r.Hostnames,
		Enabled:    r.EnabledValue(),
		Rules:      routeRuleParams(r.Rules),
	}
}

// Params 将已校验的更新请求转换为 service 参数
func (r UpdateRouteReq) Params() routeservice.UpdateRouteParams {
	return routeservice.UpdateRouteParams{
		Version:           r.Version,
		CreateRouteParams: r.CreateRouteReq.Params(),
	}
}

func routeRuleParams(rules []RouteRule) []routeservice.RouteRuleParams {
	return lo.Map(rules, func(rule RouteRule, _ int) routeservice.RouteRuleParams {
		return routeservice.RouteRuleParams{
			Name:                   rule.Name,
			PathPrefix:             rule.PathPrefix,
			Methods:                rule.Methods,
			Headers:                headerMatchParams(rule.Headers),
			Targets:                targetParams(rule.Targets),
			ModelRouting:           modelRoutingParams(rule.ModelRouting),
			RequestHeaderModifier:  headerModifierParams(rule.RequestHeaderModifier),
			ResponseHeaderModifier: headerModifierParams(rule.ResponseHeaderModifier),
			Timeout:                timeoutParams(rule.Timeout),
			Retry:                  retryParams(rule.Retry),
		}
	})
}

func modelRoutingParams(request *ModelRouting) *routeservice.ModelRoutingParams {
	if request == nil {
		return nil
	}
	return &routeservice.ModelRoutingParams{
		Models: lo.Map(request.Models, func(model ModelRoute, _ int) routeservice.ModelRouteParams {
			return routeservice.ModelRouteParams{
				Model:         model.Model,
				UpstreamID:    model.UpstreamID,
				UpstreamModel: model.UpstreamModel,
			}
		}),
	}
}

func headerMatchParams(headers []HeaderMatchReq) []routeservice.HeaderMatchParams {
	return lo.Map(headers, func(header HeaderMatchReq, _ int) routeservice.HeaderMatchParams {
		return routeservice.HeaderMatchParams{
			Name:  header.Name,
			Value: header.Value,
		}
	})
}

func targetParams(targets []RouteTarget) []routeservice.TargetParams {
	return lo.Map(targets, func(target RouteTarget, _ int) routeservice.TargetParams {
		return routeservice.TargetParams{
			UpstreamID: target.UpstreamID,
			Weight:     target.Weight,
		}
	})
}

func headerModifierParams(request *HeaderModifierReq) *routeservice.HeaderModifierParams {
	if request == nil {
		return nil
	}
	return &routeservice.HeaderModifierParams{
		Set: lo.Map(request.Set, func(header HeaderValueReq, _ int) routeservice.HeaderValueParams {
			return routeservice.HeaderValueParams{
				Name:  header.Name,
				Value: header.Value,
			}
		}),
		Remove: request.Remove,
	}
}

func timeoutParams(request *RouteTimeoutReq) *routeservice.RouteTimeoutParams {
	if request == nil {
		return nil
	}
	return &routeservice.RouteTimeoutParams{RequestMillis: request.RequestMillis}
}

func retryParams(request *RouteRetryReq) *routeservice.RouteRetryParams {
	if request == nil {
		return nil
	}
	return &routeservice.RouteRetryParams{
		Attempts:            request.Attempts,
		PerTryTimeoutMillis: request.PerTryTimeoutMillis,
	}
}
