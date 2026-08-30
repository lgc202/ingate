package mockresponsepolicy

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/internal/pkg/mockresponseconfig"
)

func validatePolicy(policy *resource.MockResponsePolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := apiregistry.ValidateResourceID(policy.Name, field.NewPath("metadata", "name"))
	errs = append(errs, apiregistry.ValidateDisplayName(
		policy.Spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	errs = append(errs, apiregistry.ValidatePolicyTargetRefs(
		policy.Spec.TargetRefs,
		specPath.Child("targetRefs"),
		resource.KindRoute,
	)...)
	if !mockresponseconfig.IsValidStatusCode(policy.Spec.StatusCode) {
		errs = append(errs, field.Invalid(
			specPath.Child("statusCode"),
			policy.Spec.StatusCode,
			fmt.Sprintf(
				"statusCode must be between %d and %d",
				mockresponseconfig.MinStatusCode,
				mockresponseconfig.MaxStatusCode,
			),
		))
	}
	normalizedContentType, valid := mockresponseconfig.NormalizeContentType(policy.Spec.ContentType)
	if !valid || normalizedContentType != policy.Spec.ContentType {
		errs = append(errs, field.Invalid(
			specPath.Child("contentType"),
			policy.Spec.ContentType,
			"contentType must be a valid media type",
		))
	}
	if len(policy.Spec.Body) > mockresponseconfig.MaxBodyBytes {
		errs = append(errs, field.Invalid(
			specPath.Child("body"),
			len(policy.Spec.Body),
			"body exceeds the supported byte length",
		))
	}
	errs = append(errs, validateHeaders(policy.Spec.Headers, specPath.Child("headers"))...)
	return errs
}

func validateHeaders(headers []resource.HeaderValue, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if len(headers) > mockresponseconfig.MaxHeaders {
		errs = append(errs, field.TooMany(path, len(headers), mockresponseconfig.MaxHeaders))
		headers = headers[:mockresponseconfig.MaxHeaders]
	}

	seen := make(map[string]bool, len(headers))
	for i, header := range headers {
		headerPath := path.Index(i)
		name := httpheader.NormalizeName(header.Name)
		if name != header.Name || !httpheader.IsValidName(name) {
			errs = append(errs, field.Invalid(headerPath.Child("name"), header.Name, "name must be a valid HTTP header name"))
		}
		if mockresponseconfig.IsReservedHeaderName(name) {
			errs = append(errs, field.Forbidden(headerPath.Child("name"), "header is managed by the HTTP response writer"))
		}
		if seen[name] {
			errs = append(errs, field.Duplicate(headerPath.Child("name"), header.Name))
		}
		seen[name] = true
		if !httpheader.IsValidValue(header.Value) {
			errs = append(errs, field.Invalid(headerPath.Child("value"), "<omitted>", "value contains invalid bytes"))
		}
	}
	return errs
}
