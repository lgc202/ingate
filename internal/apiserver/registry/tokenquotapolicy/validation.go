package tokenquotapolicy

import (
	"time"
	_ "time/tzdata"

	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

const maxTokensPerPeriod int64 = 1<<53 - 1

func validatePolicy(policy *resource.TokenQuotaPolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	var errs field.ErrorList
	if policy.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	errs = append(errs, apiregistry.ValidatePolicyTargetRefs(
		policy.Spec.TargetRefs,
		specPath.Child("targetRefs"),
		resource.KindCaller,
	)...)
	if policy.Spec.TimeZone == "" {
		errs = append(errs, field.Required(specPath.Child("timeZone"), "timeZone is required"))
	} else if _, err := time.LoadLocation(policy.Spec.TimeZone); err != nil {
		errs = append(errs, field.Invalid(specPath.Child("timeZone"), policy.Spec.TimeZone, "timeZone must be an IANA time zone"))
	}

	limitsPath := specPath.Child("limits")
	if len(policy.Spec.Limits) == 0 {
		errs = append(errs, field.Required(limitsPath, "at least one token quota limit is required"))
	}
	seen := make(map[resource.TokenQuotaPeriod]bool, len(policy.Spec.Limits))
	for i, limit := range policy.Spec.Limits {
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
		if limit.Tokens <= 0 || limit.Tokens > maxTokensPerPeriod {
			errs = append(errs, field.Invalid(limitPath.Child("tokens"), limit.Tokens, "tokens is outside the supported range"))
		}
	}
	return errs
}
