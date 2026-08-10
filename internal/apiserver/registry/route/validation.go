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
	openAIChatCompletionsPath = "/v1/chat/completions"
	aiClusterHeader           = "x-ingate-ai-cluster-v1"
)

// validateRoute 只校验资源自身结构，Gateway 和 Upstream 引用由 Controller 最终裁决
func validateRoute(route *resource.Route) field.ErrorList {
	specPath := field.NewPath("spec")
	spec := route.Spec
	errs := field.ErrorList{}

	if spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	errs = append(errs, validateGatewayRefs(spec.GatewayRefs, specPath.Child("gatewayRefs"))...)
	errs = append(errs, validateHostnames(spec.Hostnames, specPath.Child("hostnames"))...)
	errs = append(errs, validateRouteMatch(spec.Match, specPath.Child("match"))...)
	errs = append(errs, validateHeaderModifier(spec.RequestHeaderModifier, specPath.Child("requestHeaderModifier"))...)
	errs = append(errs, validateHeaderModifier(spec.ResponseHeaderModifier, specPath.Child("responseHeaderModifier"))...)
	errs = append(errs, validateForwarding(spec, specPath)...)
	errs = append(errs, validateTimeoutAndRetry(spec, specPath)...)

	if spec.ModelRouting != nil {
		errs = append(errs, validateModelRouting(spec, specPath)...)
	}
	return errs
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
	errs := field.ErrorList{}
	if len(spec.UpstreamRefs) > 0 && spec.ModelRouting != nil {
		return append(errs, field.Forbidden(path.Child("modelRouting"), "modelRouting and upstreamRefs cannot be configured together"))
	}
	if len(spec.UpstreamRefs) == 0 && spec.ModelRouting == nil {
		return append(errs, field.Required(path.Child("upstreamRefs"), "upstreamRefs or modelRouting is required"))
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

// validateModelRouting 固定 OpenAI-compatible 入口，并禁止用户覆盖 Ingate 内部选路信息
func validateModelRouting(spec resource.RouteSpec, path *field.Path) field.ErrorList {
	modelPath := path.Child("modelRouting")
	errs := field.ErrorList{}
	if spec.Match.Path.Type != resource.PathMatchExact || spec.Match.Path.Value != openAIChatCompletionsPath {
		errs = append(errs, field.Invalid(path.Child("match").Child("path"), spec.Match.Path, "model routing requires exact path /v1/chat/completions"))
	}
	if len(spec.Match.Methods) != 1 || spec.Match.Methods[0] != http.MethodPost {
		errs = append(errs, field.Invalid(path.Child("match").Child("methods"), spec.Match.Methods, "model routing requires POST as the only method"))
	}
	if spec.Retry != nil {
		errs = append(errs, field.Forbidden(path.Child("retry"), "retry is not supported by model routing"))
	}
	if len(spec.ModelRouting.Models) == 0 {
		errs = append(errs, field.Required(modelPath.Child("models"), "at least one model mapping is required"))
	}

	models := make(map[string]struct{}, len(spec.ModelRouting.Models))
	for i, model := range spec.ModelRouting.Models {
		mappingPath := modelPath.Child("models").Index(i)
		if model.Model == "" {
			errs = append(errs, field.Required(mappingPath.Child("model"), "model is required"))
		} else if _, exists := models[model.Model]; exists {
			errs = append(errs, field.Duplicate(mappingPath.Child("model"), model.Model))
		} else {
			models[model.Model] = struct{}{}
		}
		if model.UpstreamRef == "" {
			errs = append(errs, field.Required(mappingPath.Child("upstreamRef"), "upstreamRef is required"))
		}
	}
	for i, header := range spec.Match.Headers {
		if strings.EqualFold(header.Name, aiClusterHeader) {
			errs = append(errs, field.Forbidden(path.Child("match").Child("headers").Index(i), "internal AI routing headers are managed by Ingate"))
		}
	}
	if containsManagedHeader(spec.RequestHeaderModifier) {
		errs = append(errs, field.Forbidden(path.Child("requestHeaderModifier"), "AI request authentication and body framing headers are managed by Ingate"))
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

func containsManagedHeader(modifier *resource.HeaderModifier) bool {
	if modifier == nil {
		return false
	}
	for _, header := range modifier.Set {
		if isAIManagedRequestHeader(header.Name) {
			return true
		}
	}
	for _, header := range modifier.Add {
		if isAIManagedRequestHeader(header.Name) {
			return true
		}
	}
	for _, name := range modifier.Remove {
		if isAIManagedRequestHeader(name) {
			return true
		}
	}
	return false
}

func isAIManagedRequestHeader(name string) bool {
	switch strings.ToLower(name) {
	case ":authority", ":path", "accept-encoding", "anthropic-version", "authorization",
		"content-encoding", "content-length", "content-type", aiClusterHeader, "x-api-key", "x-goog-api-key":
		return true
	default:
		return false
	}
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
