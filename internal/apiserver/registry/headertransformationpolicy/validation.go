package headertransformationpolicy

import (
	"strings"

	"golang.org/x/net/http/httpguts"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

func validatePolicy(policy *resource.HeaderTransformationPolicy) field.ErrorList {
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
	if len(policy.Spec.RequestRules)+len(policy.Spec.ResponseRules) == 0 {
		errs = append(errs, field.Required(specPath, "at least one request or response rule is required"))
	}
	errs = append(errs, validateRules(policy.Spec.RequestRules, specPath.Child("requestRules"))...)
	errs = append(errs, validateRules(policy.Spec.ResponseRules, specPath.Child("responseRules"))...)
	return errs
}

func validateRules(rules []resource.HeaderTransformationRule, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	for i, rule := range rules {
		rulePath := path.Index(i)
		switch rule.Operation {
		case resource.HeaderTransformationRemove,
			resource.HeaderTransformationRename,
			resource.HeaderTransformationReplace,
			resource.HeaderTransformationAdd,
			resource.HeaderTransformationAppend:
		default:
			errs = append(errs, field.NotSupported(rulePath.Child("operation"), rule.Operation, []string{
				string(resource.HeaderTransformationRemove),
				string(resource.HeaderTransformationRename),
				string(resource.HeaderTransformationReplace),
				string(resource.HeaderTransformationAdd),
				string(resource.HeaderTransformationAppend),
			}))
		}
		if !validHeaderName(rule.Name) {
			errs = append(errs, field.Invalid(rulePath.Child("name"), rule.Name, "name must be a valid HTTP header name"))
		}
		if rule.Operation == resource.HeaderTransformationRename && !validHeaderName(rule.Value) {
			errs = append(errs, field.Invalid(rulePath.Child("value"), rule.Value, "value must be the new HTTP header name"))
		}
	}
	return errs
}

func validHeaderName(value string) bool {
	return value != "" && !strings.HasPrefix(value, ":") && httpguts.ValidHeaderFieldName(value)
}
