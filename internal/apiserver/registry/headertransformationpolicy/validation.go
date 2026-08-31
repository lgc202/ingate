package headertransformationpolicy

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/headertransformationconfig"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
)

func validatePolicy(policy *resource.HeaderTransformationPolicy) field.ErrorList {
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
	ruleCount := len(policy.Spec.RequestRules) + len(policy.Spec.ResponseRules)
	if ruleCount == 0 {
		errs = append(errs, field.Required(specPath, "at least one request or response rule is required"))
	}
	if ruleCount > headertransformationconfig.MaxRules {
		errs = append(errs, field.TooMany(specPath, ruleCount, headertransformationconfig.MaxRules))
	}
	errs = append(errs, validateRules(policy.Spec.RequestRules, specPath.Child("requestRules"))...)
	errs = append(errs, validateRules(policy.Spec.ResponseRules, specPath.Child("responseRules"))...)
	return errs
}

func validateRules(rules []resource.HeaderTransformationRule, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if len(rules) > headertransformationconfig.MaxRules {
		rules = rules[:headertransformationconfig.MaxRules]
	}
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
		if !httpheader.IsValidName(rule.Name) {
			errs = append(errs, field.Invalid(rulePath.Child("name"), rule.Name, "name must be a valid HTTP header name"))
		}
		switch rule.Operation {
		case resource.HeaderTransformationRemove:
			if rule.Value != "" {
				errs = append(errs, field.Forbidden(rulePath.Child("value"), "value is not used by Remove"))
			}
		case resource.HeaderTransformationRename:
			if !httpheader.IsValidName(rule.Value) {
				errs = append(errs, field.Invalid(rulePath.Child("value"), rule.Value, "value must be the new HTTP header name"))
			}
		case resource.HeaderTransformationReplace,
			resource.HeaderTransformationAdd,
			resource.HeaderTransformationAppend:
			if !httpheader.IsValidValue(rule.Value) {
				errs = append(errs, field.Invalid(rulePath.Child("value"), rule.Value, "value must be a valid HTTP header value"))
			}
		}
	}
	return errs
}
