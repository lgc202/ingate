package ratelimitpolicy

import (
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

func validatePolicy(policy *resource.RateLimitPolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	var errs field.ErrorList
	if policy.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	errs = append(errs, apiregistry.ValidatePolicyTargetRefs(
		policy.Spec.TargetRefs,
		specPath.Child("targetRefs"),
		resource.KindGateway,
		resource.KindRoute,
	)...)

	subjectPath := specPath.Child("subject")
	switch policy.Spec.Subject.Type {
	case resource.RateLimitSubjectShared, resource.RateLimitSubjectIP:
		if policy.Spec.Subject.HeaderName != "" {
			errs = append(errs, field.Forbidden(subjectPath.Child("headerName"), "headerName is only supported by Header subjects"))
		}
	case resource.RateLimitSubjectHeader:
		if policy.Spec.Subject.HeaderName == "" {
			errs = append(errs, field.Required(subjectPath.Child("headerName"), "headerName is required for Header subjects"))
		} else if messages := k8svalidation.IsHTTPHeaderName(policy.Spec.Subject.HeaderName); len(messages) > 0 {
			errs = append(errs, field.Invalid(subjectPath.Child("headerName"), policy.Spec.Subject.HeaderName, messages[0]))
		}
	default:
		errs = append(errs, field.NotSupported(subjectPath.Child("type"), policy.Spec.Subject.Type, []string{
			string(resource.RateLimitSubjectShared),
			string(resource.RateLimitSubjectIP),
			string(resource.RateLimitSubjectHeader),
		}))
	}

	limitPath := specPath.Child("limit")
	if policy.Spec.Limit.Requests <= 0 || policy.Spec.Limit.Requests > resource.RateLimitMaxRequests {
		errs = append(errs, field.Invalid(limitPath.Child("requests"), policy.Spec.Limit.Requests, "requests is outside the supported range"))
	}
	if policy.Spec.Limit.WindowSeconds <= 0 || policy.Spec.Limit.WindowSeconds > resource.RateLimitMaxWindowSeconds {
		errs = append(errs, field.Invalid(limitPath.Child("windowSeconds"), policy.Spec.Limit.WindowSeconds, "windowSeconds is outside the supported range"))
	}
	return errs
}
