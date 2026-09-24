package validation

import (
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

// ValidateRoute 校验新建 Route 的资源身份和期望状态。
func ValidateRoute(route *gateway.Route) field.ErrorList {
	errs := apivalidation.ValidateObjectMeta(
		&route.ObjectMeta,
		false,
		apivalidation.NameIsDNSSubdomain,
		field.NewPath("metadata"),
	)
	return append(errs, validateRouteSpec(&route.Spec, field.NewPath("spec"))...)
}

// ValidateRouteUpdate 校验更新后的 Route 资源。
func ValidateRouteUpdate(next, previous *gateway.Route) field.ErrorList {
	errs := ValidateRoute(next)
	return append(errs, apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)...)
}

// ValidateRouteStatusUpdate 校验 Route Status 子资源更新。
func ValidateRouteStatusUpdate(next, previous *gateway.Route) field.ErrorList {
	errs := apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)
	return append(errs, validateConditions(next.Status.Conditions, next.Generation)...)
}

func validateRouteSpec(spec *gateway.RouteSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, validateReferenceName(spec.GatewayName, path.Child("gatewayName"))...)
	errs = append(errs, validateListenerName(spec.ListenerName, path.Child("listenerName"))...)
	errs = append(errs, validateReferenceName(spec.UpstreamName, path.Child("upstreamName"))...)

	pathField := path.Child("path")
	if spec.Path.Type != gateway.PathMatchExact && spec.Path.Type != gateway.PathMatchPrefix {
		errs = append(errs, field.NotSupported(
			pathField.Child("type"),
			spec.Path.Type,
			[]string{string(gateway.PathMatchExact), string(gateway.PathMatchPrefix)},
		))
	}
	if !strings.HasPrefix(spec.Path.Value, "/") ||
		strings.ContainsAny(spec.Path.Value, "?#") ||
		strings.Contains(spec.Path.Value, "//") {
		errs = append(errs, field.Invalid(
			pathField.Child("value"), spec.Path.Value,
			"must be an absolute path without consecutive slashes, query, or fragment",
		))
	}

	return errs
}

func validateReferenceName(name string, path *field.Path) field.ErrorList {
	if name == "" {
		return field.ErrorList{field.Required(path, "is required")}
	}
	if messages := validation.IsDNS1123Subdomain(name); len(messages) > 0 {
		return field.ErrorList{field.Invalid(path, name, strings.Join(messages, "; "))}
	}
	return nil
}

func validateListenerName(name string, path *field.Path) field.ErrorList {
	if name == "" {
		return field.ErrorList{field.Required(path, "is required")}
	}
	if messages := validation.IsDNS1123Label(name); len(messages) > 0 {
		return field.ErrorList{field.Invalid(path, name, strings.Join(messages, "; "))}
	}
	return nil
}
