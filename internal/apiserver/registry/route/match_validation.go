package route

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

func validateGatewayRefs(refs []string, path *field.Path) field.ErrorList {
	if len(refs) == 0 {
		return field.ErrorList{field.Required(path, "at least one gatewayRef is required")}
	}
	var errs field.ErrorList
	if len(refs) > routeconfig.MaxGatewayRefs {
		errs = append(errs, field.TooMany(path, len(refs), routeconfig.MaxGatewayRefs))
	}
	seen := make(map[string]bool, len(refs))
	for i, ref := range refs {
		refPath := path.Index(i)
		if ref == "" {
			errs = append(errs, field.Required(refPath, "gatewayRef is required"))
		} else if !resourceconfig.IsCanonicalID(ref) {
			errs = append(errs, field.Invalid(refPath, ref, "gatewayRef must be a canonical UUID"))
		} else if seen[ref] {
			errs = append(errs, field.Duplicate(refPath, ref))
		} else {
			seen[ref] = true
		}
	}
	return errs
}

func validateHostnames(hostnames []string, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if len(hostnames) > routeconfig.MaxHostnames {
		errs = append(errs, field.TooMany(path, len(hostnames), routeconfig.MaxHostnames))
	}
	seen := make(map[string]bool, len(hostnames))
	for i, hostname := range hostnames {
		hostnamePath := path.Index(i)
		normalized, ok := hostnameutil.Normalize(hostname)
		if !ok || normalized == "*" {
			errs = append(errs, field.Invalid(hostnamePath, hostname, "hostname is invalid"))
			continue
		}
		if seen[normalized] {
			errs = append(errs, field.Duplicate(hostnamePath, hostname))
		} else {
			seen[normalized] = true
		}
	}
	return errs
}

func validateRouteMatch(match resource.RouteMatch, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	switch match.Path.Type {
	case resource.PathMatchPrefix, resource.PathMatchExact:
	default:
		errs = append(errs, field.NotSupported(path.Child("path", "type"), match.Path.Type, []string{
			string(resource.PathMatchPrefix),
			string(resource.PathMatchExact),
		}))
	}
	if !routeconfig.IsValidPath(match.Path.Value) {
		errs = append(errs, field.Invalid(
			path.Child("path", "value"),
			match.Path.Value,
			"path must be an absolute request path without a query or fragment",
		))
	}

	if len(match.Methods) > routeconfig.MaxHTTPMethods {
		errs = append(errs, field.TooMany(path.Child("methods"), len(match.Methods), routeconfig.MaxHTTPMethods))
	}
	seenMethods := make(map[string]bool, len(match.Methods))
	for i, method := range match.Methods {
		methodPath := path.Child("methods").Index(i)
		if !routeconfig.IsSupportedHTTPMethod(method) {
			errs = append(errs, field.NotSupported(methodPath, method, routeconfig.SupportedHTTPMethods()))
		} else if seenMethods[method] {
			errs = append(errs, field.Duplicate(methodPath, method))
		} else {
			seenMethods[method] = true
		}
	}

	if len(match.Headers) > routeconfig.MaxHeaderMatches {
		errs = append(errs, field.TooMany(path.Child("headers"), len(match.Headers), routeconfig.MaxHeaderMatches))
	}
	seenHeaders := make(map[string]bool, len(match.Headers))
	for i, header := range match.Headers {
		headerPath := path.Child("headers").Index(i)
		validName := httpheader.IsValidName(header.Name)
		if !validName {
			errs = append(errs, field.Invalid(headerPath.Child("name"), header.Name, "header name is invalid"))
		}
		if header.Value == "" || !httpheader.IsValidValue(header.Value) {
			errs = append(errs, field.Invalid(headerPath.Child("value"), header.Value, "header value is invalid"))
		}
		if validName {
			key := httpheader.NormalizeName(header.Name)
			if seenHeaders[key] {
				errs = append(errs, field.Duplicate(headerPath.Child("name"), header.Name))
			} else {
				seenHeaders[key] = true
			}
		}
	}
	return errs
}
