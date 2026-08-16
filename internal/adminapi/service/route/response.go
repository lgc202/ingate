package route

import (
	"net/http"
	"time"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func routeFromResource(route *resource.Route) *adminv1.Route {
	status := biz.EnabledResourceStatus(route.Generation, route.Spec.Enabled, route.Status.Conditions)
	response := &adminv1.Route{
		Id:                     route.Name,
		Name:                   route.Spec.DisplayName,
		Enabled:                route.Spec.Enabled,
		GatewayIds:             append([]string(nil), route.Spec.GatewayRefs...),
		Hostnames:              append([]string(nil), route.Spec.Hostnames...),
		Match:                  routeMatchFromResource(route.Spec.Match),
		HostRewrite:            hostRewriteFromResource(route.Spec.HostRewrite),
		RequestHeaderModifier:  headerModifierFromResource(route.Spec.RequestHeaderModifier),
		ResponseHeaderModifier: headerModifierFromResource(route.Spec.ResponseHeaderModifier),
		State:                  adminservice.NewResourceState(status.State),
		Message:                adminservice.ResourceMessage(status.Reason),
		Version:                route.Generation,
		CreatedAt:              adminservice.NewTimestamp(route.CreationTimestamp.Time),
		UpdatedAt:              adminservice.NewTimestamp(routeUpdatedAt(route)),
	}
	for _, upstream := range route.Spec.UpstreamRefs {
		response.Upstreams = append(response.Upstreams, &adminv1.RouteUpstream{
			UpstreamId: upstream.Name,
			Weight:     uint32(upstream.Weight),
		})
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

func hostRewriteFromResource(rewrite *resource.HostRewrite) *adminv1.HostRewrite {
	if rewrite == nil {
		return &adminv1.HostRewrite{Mode: adminv1.HostRewriteMode_HOST_REWRITE_MODE_PRESERVE}
	}

	result := &adminv1.HostRewrite{Hostname: rewrite.Hostname}
	switch rewrite.Mode {
	case resource.HostRewriteServiceAddress:
		result.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_SERVICE_ADDRESS
	case resource.HostRewritePreserve:
		result.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_PRESERVE
	case resource.HostRewriteCustom:
		result.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_CUSTOM
	default:
		result.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_UNSPECIFIED
	}
	return result
}

func routeMatchFromResource(match resource.RouteMatch) *adminv1.RouteMatch {
	response := &adminv1.RouteMatch{
		Path: &adminv1.RoutePathMatch{
			Type:  pathMatchTypeFromResource(match.Path.Type),
			Value: match.Path.Value,
		},
	}
	for _, method := range match.Methods {
		response.Methods = append(response.Methods, httpMethodFromResource(method))
	}
	for _, header := range match.Headers {
		response.Headers = append(response.Headers, &adminv1.HeaderMatch{Name: header.Name, Value: header.Value})
	}
	return response
}

func headerModifierFromResource(modifier *resource.HeaderModifier) *adminv1.HeaderModifier {
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

func pathMatchTypeFromResource(matchType resource.PathMatchType) adminv1.RoutePathMatchType {
	switch matchType {
	case resource.PathMatchPrefix:
		return adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_PREFIX
	case resource.PathMatchExact:
		return adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_EXACT
	default:
		return adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_TYPE_UNSPECIFIED
	}
}

func httpMethodFromResource(method string) adminv1.HTTPMethod {
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

func routeUpdatedAt(route *resource.Route) time.Time {
	value := route.Annotations[resource.AnnotationUpdatedAt]
	if value == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
