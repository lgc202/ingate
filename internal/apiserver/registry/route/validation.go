package route

import (
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/http/httpguts"
	"k8s.io/apimachinery/pkg/util/validation/field"

	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const (
	defaultRouteTimeoutMillis = 30000
	minRouteTimeoutMillis     = 100
	maxRouteTimeoutMillis     = 300000
	minRetryAttempts          = 1
	maxRetryAttempts          = 5
	minPerTryTimeoutMillis    = 100
	maxPerTryTimeoutMillis    = 60000
)

// validateRoute 只校验资源自身结构，Gateway 和 Upstream 引用由 Controller 最终裁决
func validateRoute(route *resource.Route) field.ErrorList {
	specPath := field.NewPath("spec")
	spec := route.Spec
	errs := field.ErrorList{}

	if spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
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

func validateHostRewrite(rewrite *resource.HostRewrite, path *field.Path) field.ErrorList {
	if rewrite == nil {
		return nil
	}

	switch rewrite.Mode {
	case resource.HostRewriteServiceAddress, resource.HostRewritePreserve:
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
			string(resource.HostRewriteServiceAddress),
			string(resource.HostRewritePreserve),
			string(resource.HostRewriteCustom),
		})}
	}
	return nil
}

func validateGatewayRefs(refs []string, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	if len(refs) == 0 {
		return append(errs, field.Required(path, "at least one gatewayRef is required"))
	}
	seen := make(map[string]struct{}, len(refs))
	for i, ref := range refs {
		refPath := path.Index(i)
		if ref == "" {
			errs = append(errs, field.Required(refPath, "gatewayRef is required"))
		} else if _, exists := seen[ref]; exists {
			errs = append(errs, field.Duplicate(refPath, ref))
		} else {
			seen[ref] = struct{}{}
		}
	}
	return errs
}

func validateHostnames(hostnames []string, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	seen := make(map[string]struct{}, len(hostnames))
	for i, hostname := range hostnames {
		hostnamePath := path.Index(i)
		normalized, ok := hostnameutil.Normalize(hostname)
		if !ok || normalized == "*" {
			errs = append(errs, field.Invalid(hostnamePath, hostname, "hostname is invalid"))
			continue
		}
		if _, exists := seen[normalized]; exists {
			errs = append(errs, field.Duplicate(hostnamePath, hostname))
		} else {
			seen[normalized] = struct{}{}
		}
	}
	return errs
}

func validateRouteMatch(match resource.RouteMatch, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	switch match.Path.Type {
	case resource.PathMatchPrefix, resource.PathMatchExact:
	default:
		errs = append(errs, field.NotSupported(path.Child("path").Child("type"), match.Path.Type, []string{
			string(resource.PathMatchPrefix),
			string(resource.PathMatchExact),
		}))
	}
	if !validPath(match.Path.Value) {
		errs = append(errs, field.Invalid(path.Child("path").Child("value"), match.Path.Value, "path must be an absolute request path without a query or fragment"))
	}

	seenMethods := make(map[string]struct{}, len(match.Methods))
	for i, method := range match.Methods {
		methodPath := path.Child("methods").Index(i)
		if !validHTTPMethod(method) {
			errs = append(errs, field.NotSupported(methodPath, method, supportedHTTPMethods()))
		} else if _, exists := seenMethods[method]; exists {
			errs = append(errs, field.Duplicate(methodPath, method))
		} else {
			seenMethods[method] = struct{}{}
		}
	}

	seenHeaders := make(map[string]struct{}, len(match.Headers))
	for i, header := range match.Headers {
		headerPath := path.Child("headers").Index(i)
		if !httpguts.ValidHeaderFieldName(header.Name) {
			errs = append(errs, field.Invalid(headerPath.Child("name"), header.Name, "header name is invalid"))
		}
		if header.Value == "" || !httpguts.ValidHeaderFieldValue(header.Value) {
			errs = append(errs, field.Invalid(headerPath.Child("value"), header.Value, "header value is invalid"))
		}
		key := strings.ToLower(header.Name)
		if _, exists := seenHeaders[key]; exists {
			errs = append(errs, field.Duplicate(headerPath.Child("name"), header.Name))
		} else {
			seenHeaders[key] = struct{}{}
		}
	}
	return errs
}

func validateForwarding(spec resource.RouteSpec, path *field.Path) field.ErrorList {
	if spec.AI != nil {
		return validateAIForwarding(spec, path)
	}

	errs := field.ErrorList{}
	if len(spec.UpstreamRefs) == 0 {
		return append(errs, field.Required(path.Child("upstreamRefs"), "at least one upstreamRef is required"))
	}

	seen := make(map[string]struct{}, len(spec.UpstreamRefs))
	for i, ref := range spec.UpstreamRefs {
		refPath := path.Child("upstreamRefs").Index(i)
		if ref.Name == "" {
			errs = append(errs, field.Required(refPath.Child("name"), "upstreamRef.name is required"))
		} else if _, exists := seen[ref.Name]; exists {
			errs = append(errs, field.Duplicate(refPath.Child("name"), ref.Name))
		} else {
			seen[ref.Name] = struct{}{}
		}
		if ref.Weight < 1 || ref.Weight > 1000 {
			errs = append(errs, field.Invalid(refPath.Child("weight"), ref.Weight, "upstreamRef.weight must be between 1 and 1000"))
		}
	}
	return errs
}

func validateAIForwarding(spec resource.RouteSpec, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	if len(spec.UpstreamRefs) != 0 {
		errs = append(errs, field.Forbidden(path.Child("upstreamRefs"), "AI route uses ai.models targets"))
	}
	if len(spec.Match.Methods) != 1 || strings.ToUpper(spec.Match.Methods[0]) != http.MethodPost {
		errs = append(errs, field.Invalid(path.Child("match", "methods"), spec.Match.Methods, "AI route currently requires POST"))
	}
	if len(spec.AI.Models) == 0 {
		return append(errs, field.Required(path.Child("ai", "models"), "at least one client model is required"))
	}

	modelsPath := path.Child("ai", "models")
	seenModels := make(map[string]struct{}, len(spec.AI.Models))
	for i, model := range spec.AI.Models {
		modelPath := modelsPath.Index(i)
		if model.Name == "" || strings.TrimSpace(model.Name) != model.Name {
			errs = append(errs, field.Invalid(modelPath.Child("name"), model.Name, "client model must be a non-empty trimmed string"))
		} else if _, exists := seenModels[model.Name]; exists {
			errs = append(errs, field.Duplicate(modelPath.Child("name"), model.Name))
		} else {
			seenModels[model.Name] = struct{}{}
		}

		if len(model.Targets) == 0 {
			errs = append(errs, field.Required(modelPath.Child("targets"), "at least one model target is required"))
			continue
		}
		seenTargets := make(map[string]struct{}, len(model.Targets))
		for j, target := range model.Targets {
			targetPath := modelPath.Child("targets").Index(j)
			if target.UpstreamRef == "" {
				errs = append(errs, field.Required(targetPath.Child("upstreamRef"), "upstreamRef is required"))
			} else if _, exists := seenTargets[target.UpstreamRef]; exists {
				errs = append(errs, field.Duplicate(targetPath.Child("upstreamRef"), target.UpstreamRef))
			} else {
				seenTargets[target.UpstreamRef] = struct{}{}
			}
			if target.Model == "" || strings.TrimSpace(target.Model) != target.Model {
				errs = append(errs, field.Invalid(targetPath.Child("model"), target.Model, "upstream model must be a non-empty trimmed string"))
			}
			if target.Weight < 1 || target.Weight > 1000 {
				errs = append(errs, field.Invalid(targetPath.Child("weight"), target.Weight, "weight must be between 1 and 1000"))
			}
		}
	}
	return errs
}

func validateTimeoutAndRetry(spec resource.RouteSpec, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	requestTimeout := defaultRouteTimeoutMillis
	if spec.Timeout != nil {
		requestTimeout = spec.Timeout.RequestMillis
		if requestTimeout < minRouteTimeoutMillis || requestTimeout > maxRouteTimeoutMillis {
			errs = append(errs, field.Invalid(path.Child("timeout").Child("requestMillis"), requestTimeout, "timeout.requestMillis is out of range"))
		}
	}
	if spec.Retry == nil {
		return errs
	}
	if spec.Retry.Attempts < minRetryAttempts || spec.Retry.Attempts > maxRetryAttempts {
		errs = append(errs, field.Invalid(path.Child("retry").Child("attempts"), spec.Retry.Attempts, "retry.attempts is out of range"))
	}
	if spec.Retry.PerTryTimeoutMillis < minPerTryTimeoutMillis || spec.Retry.PerTryTimeoutMillis > maxPerTryTimeoutMillis {
		errs = append(errs, field.Invalid(path.Child("retry").Child("perTryTimeoutMillis"), spec.Retry.PerTryTimeoutMillis, "retry.perTryTimeoutMillis is out of range"))
	}
	if spec.Retry.PerTryTimeoutMillis > requestTimeout {
		errs = append(errs, field.Invalid(path.Child("retry").Child("perTryTimeoutMillis"), spec.Retry.PerTryTimeoutMillis, "retry.perTryTimeoutMillis must not exceed timeout.requestMillis"))
	}
	return errs
}

func validateHeaderModifier(modifier *resource.HeaderModifier, path *field.Path) field.ErrorList {
	if modifier == nil {
		return nil
	}
	errs := field.ErrorList{}
	if len(modifier.Set) == 0 && len(modifier.Add) == 0 && len(modifier.Remove) == 0 {
		return append(errs, field.Required(path, "at least one header modifier action is required"))
	}
	seen := make(map[string]struct{}, len(modifier.Set)+len(modifier.Add)+len(modifier.Remove))
	errs = append(errs, validateHeaderValues(modifier.Set, path.Child("set"), seen)...)
	errs = append(errs, validateHeaderValues(modifier.Add, path.Child("add"), seen)...)
	for i, name := range modifier.Remove {
		namePath := path.Child("remove").Index(i)
		if !httpguts.ValidHeaderFieldName(name) {
			errs = append(errs, field.Invalid(namePath, name, "header name is invalid"))
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			errs = append(errs, field.Duplicate(namePath, name))
		} else {
			seen[key] = struct{}{}
		}
	}
	return errs
}

func validateHeaderValues(values []resource.HeaderValue, path *field.Path, seen map[string]struct{}) field.ErrorList {
	errs := field.ErrorList{}
	for i, value := range values {
		valuePath := path.Index(i)
		if !httpguts.ValidHeaderFieldName(value.Name) {
			errs = append(errs, field.Invalid(valuePath.Child("name"), value.Name, "header name is invalid"))
		}
		if value.Value == "" || !httpguts.ValidHeaderFieldValue(value.Value) {
			errs = append(errs, field.Invalid(valuePath.Child("value"), value.Value, "header value is invalid"))
		}
		key := strings.ToLower(value.Name)
		if _, exists := seen[key]; exists {
			errs = append(errs, field.Duplicate(valuePath.Child("name"), value.Name))
		} else {
			seen[key] = struct{}{}
		}
	}
	return errs
}

func validPath(value string) bool {
	if !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "?#") {
		return false
	}
	_, err := url.ParseRequestURI(value)
	return err == nil
}

func validHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func supportedHTTPMethods() []string {
	return []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	}
}
