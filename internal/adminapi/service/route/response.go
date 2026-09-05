package route

import (
	"slices"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func routeResponse(route *resource.Route) *adminv1.Route {
	status := resourceview.EnabledStatus(route.Generation, route.Spec.Enabled, route.Status.Conditions)
	config := &adminv1.RouteConfig{
		Enabled:                new(route.Spec.Enabled),
		AccessMode:             accessModeResponse(route.Spec.AccessMode),
		GatewayIds:             slices.Clone(route.Spec.GatewayRefs),
		Hostnames:              slices.Clone(route.Spec.Hostnames),
		Match:                  matchResponse(route.Spec.Match),
		Forwarding:             forwardingResponse(route.Spec),
		HostRewrite:            hostRewriteResponse(route.Spec.HostRewrite),
		RequestHeaderModifier:  headerModifierResponse(route.Spec.RequestHeaderModifier),
		ResponseHeaderModifier: headerModifierResponse(route.Spec.ResponseHeaderModifier),
		Timeout:                &adminv1.RouteTimeout{RequestMillis: uint32(route.Spec.Timeout.RequestMillis)},
	}
	if route.Spec.Retry != nil {
		config.Retry = &adminv1.RouteRetry{
			Attempts:            uint32(route.Spec.Retry.Attempts),
			PerTryTimeoutMillis: uint32(route.Spec.Retry.PerTryTimeoutMillis),
		}
	}
	return &adminv1.Route{
		Id:        route.Name,
		Name:      route.Spec.DisplayName,
		Config:    config,
		State:     adminservice.ResourceState(status.State),
		Message:   adminservice.ResourceMessage(status.Reason),
		Version:   route.Generation,
		CreatedAt: adminservice.Timestamp(route.CreationTimestamp.Time),
		UpdatedAt: adminservice.Timestamp(adminservice.ResourceUpdatedAt(route.Annotations)),
	}
}

func accessModeResponse(mode resource.RouteAccessMode) adminv1.RouteAccessMode {
	switch mode {
	case resource.RouteAccessPublic:
		return adminv1.RouteAccessMode_ROUTE_ACCESS_MODE_PUBLIC
	case resource.RouteAccessCaller:
		return adminv1.RouteAccessMode_ROUTE_ACCESS_MODE_CALLER
	default:
		return adminv1.RouteAccessMode_ROUTE_ACCESS_MODE_UNSPECIFIED
	}
}

func hostRewriteResponse(rewrite resource.HostRewrite) *adminv1.HostRewrite {
	response := &adminv1.HostRewrite{Hostname: rewrite.Hostname}
	switch rewrite.Mode {
	case resource.HostRewriteUpstreamHost:
		response.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_SERVICE_HOST
	case resource.HostRewritePreserve:
		response.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_PRESERVE
	case resource.HostRewriteCustom:
		response.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_CUSTOM
	default:
		response.Mode = adminv1.HostRewriteMode_HOST_REWRITE_MODE_UNSPECIFIED
	}
	return response
}
