package route

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// validateRoute 只校验资源自身结构，Gateway 和 Upstream 引用由 Controller 最终裁决。
func validateRoute(route *resource.Route) field.ErrorList {
	specPath := field.NewPath("spec")
	spec := route.Spec
	errs := apiregistry.ValidateResourceID(route.Name, field.NewPath("metadata", "name"))

	errs = append(errs, apiregistry.ValidateDisplayName(
		spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	switch spec.AccessMode {
	case resource.RouteAccessPublic, resource.RouteAccessCaller:
	default:
		errs = append(errs, field.NotSupported(specPath.Child("accessMode"), spec.AccessMode, []string{
			string(resource.RouteAccessPublic),
			string(resource.RouteAccessCaller),
		}))
	}
	errs = append(errs, validateGatewayRefs(spec.GatewayRefs, specPath.Child("gatewayRefs"))...)
	errs = append(errs, validateHostnames(spec.Hostnames, specPath.Child("hostnames"))...)
	errs = append(errs, validateRouteMatch(spec.Match, specPath.Child("match"))...)
	errs = append(errs, validateHostRewrite(spec.HostRewrite, specPath.Child("hostRewrite"))...)
	errs = append(errs, validateHeaderModifier(spec.RequestHeaderModifier, specPath.Child("requestHeaderModifier"))...)
	errs = append(errs, validateHeaderModifier(spec.ResponseHeaderModifier, specPath.Child("responseHeaderModifier"))...)
	errs = append(errs, validateForwarding(spec, specPath)...)
	errs = append(errs, validateTimeoutAndRetry(spec, specPath)...)
	return errs
}

func validateHostRewrite(rewrite resource.HostRewrite, path *field.Path) field.ErrorList {
	switch rewrite.Mode {
	case resource.HostRewriteUpstreamHost, resource.HostRewritePreserve:
		if rewrite.Hostname != "" {
			return field.ErrorList{field.Forbidden(path.Child("hostname"), "hostname is only valid in Custom mode")}
		}
	case resource.HostRewriteCustom:
		hostname, ok := hostnameutil.Normalize(rewrite.Hostname)
		if !ok || hostname == "*" {
			return field.ErrorList{field.Invalid(path.Child("hostname"), rewrite.Hostname, "hostname is invalid")}
		}
	default:
		return field.ErrorList{field.NotSupported(path.Child("mode"), rewrite.Mode, []string{
			string(resource.HostRewriteUpstreamHost),
			string(resource.HostRewritePreserve),
			string(resource.HostRewriteCustom),
		})}
	}
	return nil
}

func validateTimeoutAndRetry(spec resource.RouteSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	requestTimeoutMillis := spec.Timeout.RequestMillis
	if requestTimeoutMillis < routeconfig.MinRequestTimeoutMillis ||
		requestTimeoutMillis > routeconfig.MaxRequestTimeoutMillis {
		errs = append(errs, field.Invalid(
			path.Child("timeout", "requestMillis"),
			requestTimeoutMillis,
			"timeout.requestMillis is out of range",
		))
	}
	if spec.Retry == nil {
		return errs
	}
	if spec.Retry.Attempts < routeconfig.MinRetryAttempts ||
		spec.Retry.Attempts > routeconfig.MaxRetryAttempts {
		errs = append(errs, field.Invalid(
			path.Child("retry", "attempts"),
			spec.Retry.Attempts,
			"retry.attempts is out of range",
		))
	}
	if spec.Retry.PerTryTimeoutMillis < routeconfig.MinPerTryTimeoutMillis ||
		spec.Retry.PerTryTimeoutMillis > routeconfig.MaxPerTryTimeoutMillis {
		errs = append(errs, field.Invalid(
			path.Child("retry", "perTryTimeoutMillis"),
			spec.Retry.PerTryTimeoutMillis,
			"retry.perTryTimeoutMillis is out of range",
		))
	}
	if spec.Retry.PerTryTimeoutMillis > requestTimeoutMillis {
		errs = append(errs, field.Invalid(
			path.Child("retry", "perTryTimeoutMillis"),
			spec.Retry.PerTryTimeoutMillis,
			"retry.perTryTimeoutMillis must not exceed timeout.requestMillis",
		))
	}
	return errs
}
