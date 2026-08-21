package route

import (
	"net/http"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func routeResponse(route *resource.Route) *adminv1.Route {
	status := biz.EnabledResourceStatus(route.Generation, route.Spec.Enabled, route.Status.Conditions)
	response := &adminv1.Route{
		Id:                     route.Name,
		Name:                   route.Spec.DisplayName,
		Enabled:                route.Spec.Enabled,
		AccessMode:             routeAccessModeResponse(route.Spec.AccessMode),
		GatewayIds:             append([]string(nil), route.Spec.GatewayRefs...),
		Hostnames:              append([]string(nil), route.Spec.Hostnames...),
		Match:                  routeMatchResponse(route.Spec.Match),
		HostRewrite:            hostRewriteResponse(route.Spec.HostRewrite),
		RequestHeaderModifier:  headerModifierResponse(route.Spec.RequestHeaderModifier),
		ResponseHeaderModifier: headerModifierResponse(route.Spec.ResponseHeaderModifier),
		State:                  adminservice.ResourceState(status.State),
		Message:                adminservice.ResourceMessage(status.Reason),
		Version:                route.Generation,
		CreatedAt:              adminservice.Timestamp(route.CreationTimestamp.Time),
		UpdatedAt:              adminservice.Timestamp(adminservice.ResourceUpdatedAt(route.Annotations)),
	}
	for _, upstream := range route.Spec.UpstreamRefs {
		response.Upstreams = append(response.Upstreams, &adminv1.RouteUpstream{
			UpstreamId: upstream.Name,
			Weight:     uint32(upstream.Weight),
		})
	}
	if route.Spec.AI != nil {
		response.Ai = aiRouteResponse(route.Spec.AI)
	}
	if route.Spec.Timeout != nil {
		response.Timeout = &adminv1.RouteTimeout{RequestMillis: uint32(route.Spec.Timeout.RequestMillis)}
	}
	if route.Spec.Retry != nil {
		response.Retry = &adminv1.RouteRetry{
			Attempts:            uint32(route.Spec.Retry.Attempts),
			PerTryTimeoutMillis: uint32(route.Spec.Retry.PerTryTimeoutMillis),
		}
	}
	return response
}

func routeAccessModeResponse(mode resource.RouteAccessMode) adminv1.RouteAccessMode {
	switch mode {
	case resource.RouteAccessPublic:
		return adminv1.RouteAccessMode_ROUTE_ACCESS_PUBLIC
	case resource.RouteAccessCaller:
		return adminv1.RouteAccessMode_ROUTE_ACCESS_CALLER
	default:
		return adminv1.RouteAccessMode_ROUTE_ACCESS_MODE_UNSPECIFIED
	}
}

func aiRouteResponse(ai *resource.AIRoute) *adminv1.AIRoute {
	response := &adminv1.AIRoute{Models: make([]*adminv1.AIModel, 0, len(ai.Models))}
	for _, model := range ai.Models {
		modelResponse := &adminv1.AIModel{
			Name:    model.Name,
			Targets: make([]*adminv1.AIModelTarget, 0, len(model.Targets)),
		}
		for _, target := range model.Targets {
			modelResponse.Targets = append(modelResponse.Targets, &adminv1.AIModelTarget{
				UpstreamId: target.UpstreamRef,
				Model:      target.Model,
				Weight:     uint32(target.Weight),
			})
		}
		response.Models = append(response.Models, modelResponse)
	}
	return response
}

func hostRewriteResponse(rewrite *resource.HostRewrite) *adminv1.HostRewrite {
	if rewrite == nil {
		return &adminv1.HostRewrite{Mode: adminv1.HostRewriteMode_HOST_REWRITE_MODE_PRESERVE}
	}

	response := &adminv1.HostRewrite{Hostname: rewrite.Hostname}
	switch rewrite.Mode {
	case resource.HostRewriteServiceAddress:
		response.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_SERVICE_ADDRESS
	case resource.HostRewritePreserve:
		response.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_PRESERVE
	case resource.HostRewriteCustom:
		response.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_CUSTOM
	default:
		response.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_UNSPECIFIED
	}
	return response
}

func routeMatchResponse(match resource.RouteMatch) *adminv1.RouteMatch {
	response := &adminv1.RouteMatch{
		Path: &adminv1.RoutePathMatch{
			Type:  pathMatchTypeResponse(match.Path.Type),
			Value: match.Path.Value,
		},
	}
	for _, method := range match.Methods {
		response.Methods = append(response.Methods, httpMethodResponse(method))
	}
	for _, header := range match.Headers {
		response.Headers = append(response.Headers, &adminv1.HeaderMatch{Name: header.Name, Value: header.Value})
	}
	return response
}

func headerModifierResponse(modifier *resource.HeaderModifier) *adminv1.HeaderModifier {
	if modifier == nil {
		return nil
	}
	response := &adminv1.HeaderModifier{Remove: append([]string(nil), modifier.Remove...)}
	for _, header := range modifier.Set {
		response.Set = append(response.Set, &adminv1.HeaderValue{Name: header.Name, Value: header.Value})
	}
	for _, header := range modifier.Add {
		response.Add = append(response.Add, &adminv1.HeaderValue{Name: header.Name, Value: header.Value})
	}
	return response
}

func pathMatchTypeResponse(matchType resource.PathMatchType) adminv1.RoutePathMatchType {
	switch matchType {
	case resource.PathMatchPrefix:
		return adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_PREFIX
	case resource.PathMatchExact:
		return adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_EXACT
	default:
		return adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_TYPE_UNSPECIFIED
	}
}

func httpMethodResponse(method string) adminv1.HTTPMethod {
	switch method {
	case http.MethodGet:
		return adminv1.HTTPMethod_HTTP_METHOD_GET
	case http.MethodHead:
		return adminv1.HTTPMethod_HTTP_METHOD_HEAD
	case http.MethodPost:
		return adminv1.HTTPMethod_HTTP_METHOD_POST
	case http.MethodPut:
		return adminv1.HTTPMethod_HTTP_METHOD_PUT
	case http.MethodPatch:
		return adminv1.HTTPMethod_HTTP_METHOD_PATCH
	case http.MethodDelete:
		return adminv1.HTTPMethod_HTTP_METHOD_DELETE
	case http.MethodOptions:
		return adminv1.HTTPMethod_HTTP_METHOD_OPTIONS
	default:
		return adminv1.HTTPMethod_HTTP_METHOD_UNSPECIFIED
	}
}
