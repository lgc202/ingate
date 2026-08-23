package mockresponsepolicy

import (
	"mime"
	"strings"

	"golang.org/x/net/http/httpguts"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

const maxResponseBodyBytes = 1 << 20

func validatePolicy(policy *resource.MockResponsePolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	var errs field.ErrorList
	if policy.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	errs = append(errs, apiregistry.ValidatePolicyTargetRefs(
		policy.Spec.TargetRefs,
		specPath.Child("targetRefs"),
		resource.KindRoute,
	)...)
	if policy.Spec.StatusCode < 200 || policy.Spec.StatusCode > 599 {
		errs = append(errs, field.Invalid(specPath.Child("statusCode"), policy.Spec.StatusCode, "statusCode must be between 200 and 599"))
	}
	if _, _, err := mime.ParseMediaType(policy.Spec.ContentType); err != nil {
		errs = append(errs, field.Invalid(specPath.Child("contentType"), policy.Spec.ContentType, "contentType must be a valid media type"))
	}
	if len(policy.Spec.Body) > maxResponseBodyBytes {
		errs = append(errs, field.TooLong(specPath.Child("body"), policy.Spec.Body, maxResponseBodyBytes))
	}
	errs = append(errs, validateHeaders(policy.Spec.Headers, specPath.Child("headers"))...)
	return errs
}

func validateHeaders(headers []resource.HeaderValue, path *field.Path) field.ErrorList {
	seen := make(map[string]struct{}, len(headers))
	var errs field.ErrorList
	for i, header := range headers {
		headerPath := path.Index(i)
		name := strings.ToLower(header.Name)
		if name == "" || strings.HasPrefix(name, ":") || !httpguts.ValidHeaderFieldName(name) {
			errs = append(errs, field.Invalid(headerPath.Child("name"), header.Name, "name must be a valid HTTP header name"))
		}
		if name == "content-type" {
			errs = append(errs, field.Forbidden(headerPath.Child("name"), "content-type is configured by contentType"))
		}
		if _, exists := seen[name]; exists {
			errs = append(errs, field.Duplicate(headerPath.Child("name"), header.Name))
		}
		seen[name] = struct{}{}
		if !httpguts.ValidHeaderFieldValue(header.Value) {
			errs = append(errs, field.Invalid(headerPath.Child("value"), header.Value, "value contains invalid bytes"))
		}
	}
	return errs
}
