package tokenquota

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	policyregistry "github.com/lgc202/ingate/internal/apiserver/registry/policy"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	apivalidation "github.com/lgc202/ingate/internal/pkg/apis/gateway/validation"
)

func validatePolicy(policy *resource.TokenQuotaPolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := apiregistry.ValidateResourceID(policy.Name, field.NewPath("metadata", "name"))
	errs = append(errs, apiregistry.ValidateDisplayName(
		policy.Spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	errs = append(errs, policyregistry.ValidateTargets(
		policy.Spec.TargetRefs,
		specPath.Child("targetRefs"),
		resource.KindCaller,
	)...)
	canonicalTimeZone, _, valid := apivalidation.LoadLocation(policy.Spec.TimeZone)
	if !valid || canonicalTimeZone != policy.Spec.TimeZone {
		errs = append(errs, field.Invalid(
			specPath.Child("timeZone"),
			policy.Spec.TimeZone,
			"timeZone must be an IANA time zone",
		))
	}

	limitsPath := specPath.Child("limits")
	if len(policy.Spec.Limits) == 0 {
		errs = append(errs, field.Required(limitsPath, "at least one token quota limit is required"))
	}
	limits := policy.Spec.Limits
	if len(limits) > apivalidation.MaxLimits {
		errs = append(errs, field.TooMany(limitsPath, len(limits), apivalidation.MaxLimits))
		limits = limits[:apivalidation.MaxLimits]
	}
	seen := make(map[resource.TokenQuotaPeriod]bool, len(limits))
	for i, limit := range limits {
		limitPath := limitsPath.Index(i)
		switch limit.Period {
		case resource.TokenQuotaPeriodDay, resource.TokenQuotaPeriodWeek, resource.TokenQuotaPeriodMonth:
		default:
			errs = append(errs, field.NotSupported(limitPath.Child("period"), limit.Period, []string{
				string(resource.TokenQuotaPeriodDay),
				string(resource.TokenQuotaPeriodWeek),
				string(resource.TokenQuotaPeriodMonth),
			}))
		}
		if seen[limit.Period] {
			errs = append(errs, field.Duplicate(limitPath.Child("period"), limit.Period))
		}
		seen[limit.Period] = true
		if !apivalidation.IsValidTokenLimit(limit.Tokens) {
			errs = append(errs, field.Invalid(limitPath.Child("tokens"), limit.Tokens, "tokens is outside the supported range"))
		}
	}
	return errs
}
