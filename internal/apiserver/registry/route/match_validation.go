package route

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	apivalidation "github.com/lgc202/ingate/internal/pkg/apis/gateway/validation"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
)

func validateGatewayRefs(refs []string, path *field.Path) field.ErrorList {
	if len(refs) == 0 {
		return field.ErrorList{field.Required(path, "at least one gatewayRef is required")}
	}
	var errs field.ErrorList
	if len(refs) > apivalidation.MaxGatewayRefs {
		errs = append(errs, field.TooMany(path, len(refs), apivalidation.MaxGatewayRefs))
		refs = refs[:apivalidation.MaxGatewayRefs]
	}
	seen := make(map[string]bool, len(refs))
	for i, ref := range refs {
		refPath := path.Index(i)
		if ref == "" {
			errs = append(errs, field.Required(refPath, "gatewayRef is required"))
		} else if !apivalidation.IsCanonicalID(ref) {
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
	if len(hostnames) > apivalidation.MaxHostnames {
		errs = append(errs, field.TooMany(path, len(hostnames), apivalidation.MaxHostnames))
		hostnames = hostnames[:apivalidation.MaxHostnames]
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
	if !apivalidation.IsValidPath(match.Path.Value) {
		errs = append(errs, field.Invalid(
			path.Child("path", "value"),
			match.Path.Value,
			"path must be an absolute request path without a query or fragment",
		))
	}

	methods := match.Methods
	if len(methods) > apivalidation.MaxHTTPMethods {
		errs = append(errs, field.TooMany(path.Child("methods"), len(methods), apivalidation.MaxHTTPMethods))
		methods = methods[:apivalidation.MaxHTTPMethods]
	}
	seenMethods := make(map[string]bool, len(methods))
	for i, method := range methods {
		methodPath := path.Child("methods").Index(i)
		if !apivalidation.IsSupportedHTTPMethod(method) {
			errs = append(errs, field.NotSupported(methodPath, method, apivalidation.SupportedHTTPMethods()))
		} else if seenMethods[method] {
			errs = append(errs, field.Duplicate(methodPath, method))
		} else {
			seenMethods[method] = true
		}
	}

	headers := match.Headers
	if len(headers) > apivalidation.MaxHeaderMatches {
		errs = append(errs, field.TooMany(path.Child("headers"), len(headers), apivalidation.MaxHeaderMatches))
		headers = headers[:apivalidation.MaxHeaderMatches]
	}
	seenHeaders := make(map[string]bool, len(headers))
	for i, header := range headers {
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
