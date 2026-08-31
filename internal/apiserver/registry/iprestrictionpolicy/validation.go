package iprestrictionpolicy

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/iprestrictionconfig"
)

func validatePolicy(policy *resource.IPRestrictionPolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := apiregistry.ValidateResourceID(policy.Name, field.NewPath("metadata", "name"))
	errs = append(errs, apiregistry.ValidateDisplayName(
		policy.Spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	errs = append(errs, apiregistry.ValidatePolicyTargetRefs(
		policy.Spec.TargetRefs,
		specPath.Child("targetRefs"),
		resource.KindGateway,
		resource.KindRoute,
	)...)

	hasAllow := len(policy.Spec.Allow) > 0
	hasDeny := len(policy.Spec.Deny) > 0
	if hasAllow == hasDeny {
		errs = append(errs, field.Invalid(specPath, policy.Spec, "exactly one of allow or deny must be configured"))
	}
	errs = append(errs, validateRanges(policy.Spec.Allow, specPath.Child("allow"))...)
	errs = append(errs, validateRanges(policy.Spec.Deny, specPath.Child("deny"))...)
	return errs
}

func validateRanges(values []string, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if len(values) > iprestrictionconfig.MaxRanges {
		errs = append(errs, field.TooMany(path, len(values), iprestrictionconfig.MaxRanges))
		values = values[:iprestrictionconfig.MaxRanges]
	}
	for i, value := range values {
		normalized, valid := iprestrictionconfig.NormalizeRange(value)
		if !valid || normalized != value {
			errs = append(errs, field.Invalid(path.Index(i), value, "value must be an IP address or CIDR prefix"))
		}
	}
	return errs
}
