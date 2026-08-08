package route

import (
	"strconv"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func routeReply(route *resource.Route) *adminv1.Route {
	status := biz.EnabledResourceStatus(route.Generation, route.Spec.Enabled, route.Status.Conditions)
	reply := &adminv1.Route{
		Id: route.Name, Version: strconv.FormatInt(route.Generation, 10), Status: adminservice.ResourceStatus(status),
		Name: route.Spec.DisplayName, Hostnames: append([]string(nil), route.Spec.Hostnames...),
		Enabled: route.Spec.Enabled, CreatedAt: adminservice.Timestamp(route.CreationTimestamp.Time),
	}
	for _, ref := range route.Spec.ParentRefs {
		reply.GatewayIds = append(reply.GatewayIds, ref.Name)
	}
	for _, rule := range route.Spec.Rules {
		reply.Rules = append(reply.Rules, routeRuleReply(rule))
	}
	return reply
}

func routeRuleReply(rule resource.RouteRule) *adminv1.RouteRule {
	reply := &adminv1.RouteRule{
		Name: rule.Name, PathPrefix: rule.PathPrefix, Methods: append([]string(nil), rule.Methods...),
	}
	for _, header := range rule.Headers {
		reply.Headers = append(reply.Headers, &adminv1.HeaderMatch{Name: header.Name, Value: header.Value})
	}
	for _, target := range rule.UpstreamRefs {
		reply.Targets = append(reply.Targets, &adminv1.RouteTarget{UpstreamId: target.Name, Weight: int32(target.Weight)})
	}
	if rule.ModelRouting != nil {
		reply.ModelRouting = &adminv1.ModelRouting{}
		for _, model := range rule.ModelRouting.Models {
			reply.ModelRouting.Models = append(reply.ModelRouting.Models, &adminv1.ModelRoute{
				Model: model.Model, UpstreamId: model.UpstreamRef, UpstreamModel: model.UpstreamModel,
			})
		}
	}
	for _, filter := range rule.Filters {
		switch filter.Type {
		case resource.RouteFilterRequestHeaderModifier:
			reply.RequestHeaderModifier = headerModifierReply(filter.RequestHeaderModifier)
		case resource.RouteFilterResponseHeaderModifier:
			reply.ResponseHeaderModifier = headerModifierReply(filter.ResponseHeaderModifier)
		}
	}
	if rule.Timeout != nil {
		reply.Timeout = &adminv1.RouteTimeout{RequestMillis: int32(rule.Timeout.RequestMillis)}
	}
	if rule.Retry != nil {
		reply.Retry = &adminv1.RouteRetry{
			Attempts: int32(rule.Retry.Attempts), PerTryTimeoutMillis: int32(rule.Retry.PerTryTimeoutMillis),
		}
	}
	return reply
}

func headerModifierReply(modifier *resource.HeaderModifier) *adminv1.HeaderModifier {
	if modifier == nil {
		return nil
	}
	reply := &adminv1.HeaderModifier{Remove: append([]string(nil), modifier.Remove...)}
	for _, header := range modifier.Set {
		reply.Set = append(reply.Set, &adminv1.HeaderValue{Name: header.Name, Value: header.Value})
	}
	return reply
}
