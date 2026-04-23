package validation

import (
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

var (
	validAuthTypes      = map[string]struct{}{"JWT": {}, "APIKey": {}}
	validAuthTargets    = map[string]struct{}{"Gateway": {}, "Route": {}}
	validTrafficTargets = map[string]struct{}{"Gateway": {}, "Route": {}, "Backend": {}}
	validRateLimitUnits = map[string]struct{}{"second": {}, "minute": {}, "hour": {}}
)

func ValidateAuthPolicy(policy *policyv1alpha1.AuthPolicy) field.ErrorList {
	var allErrs field.ErrorList

	if len(policy.Spec.TargetRefs) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "targetRefs"), "at least one targetRef is required"))
	}
	for i, targetRef := range policy.Spec.TargetRefs {
		targetPath := field.NewPath("spec", "targetRefs").Index(i)
		if _, ok := validAuthTargets[targetRef.Kind]; !ok {
			allErrs = append(allErrs, field.NotSupported(targetPath.Child("kind"), targetRef.Kind, []string{"Gateway", "Route"}))
		}
		if targetRef.Name == "" {
			allErrs = append(allErrs, field.Required(targetPath.Child("name"), "targetRef name is required"))
		}
	}

	if _, ok := validAuthTypes[policy.Spec.Type]; !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "type"), policy.Spec.Type, []string{"JWT", "APIKey"}))
	}

	switch policy.Spec.Type {
	case "JWT":
		if policy.Spec.JWT == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "jwt"), "jwt config is required"))
		}
		if policy.Spec.APIKey != nil {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "apiKey"), "apiKey config is not allowed for JWT auth"))
		}
		if policy.Spec.JWT != nil {
			if policy.Spec.JWT.Issuer == "" {
				allErrs = append(allErrs, field.Required(field.NewPath("spec", "jwt", "issuer"), "issuer is required"))
			}
			if len(policy.Spec.JWT.FromHeaders) == 0 {
				allErrs = append(allErrs, field.Required(field.NewPath("spec", "jwt", "fromHeaders"), "at least one header source is required"))
			}
			for i, src := range policy.Spec.JWT.FromHeaders {
				if src.Name == "" {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "jwt", "fromHeaders").Index(i).Child("name"), "header name is required"))
				}
			}
		}
	case "APIKey":
		if policy.Spec.APIKey == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "apiKey"), "apiKey config is required"))
		}
		if policy.Spec.JWT != nil {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "jwt"), "jwt config is not allowed for APIKey auth"))
		}
		if policy.Spec.APIKey != nil {
			if len(policy.Spec.APIKey.FromHeaders) == 0 {
				allErrs = append(allErrs, field.Required(field.NewPath("spec", "apiKey", "fromHeaders"), "at least one header source is required"))
			}
			for i, src := range policy.Spec.APIKey.FromHeaders {
				if src.Name == "" {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "apiKey", "fromHeaders").Index(i).Child("name"), "header name is required"))
				}
			}
		}
	}

	return allErrs
}

func ValidateAuthPolicyUpdate(update, old *policyv1alpha1.AuthPolicy) field.ErrorList {
	return ValidateAuthPolicy(update)
}

func ValidateTrafficPolicy(policy *policyv1alpha1.TrafficPolicy) field.ErrorList {
	var allErrs field.ErrorList

	if len(policy.Spec.TargetRefs) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "targetRefs"), "at least one targetRef is required"))
	}
	for i, targetRef := range policy.Spec.TargetRefs {
		targetPath := field.NewPath("spec", "targetRefs").Index(i)
		if _, ok := validTrafficTargets[targetRef.Kind]; !ok {
			allErrs = append(allErrs, field.NotSupported(targetPath.Child("kind"), targetRef.Kind, []string{"Gateway", "Route", "Backend"}))
		}
		if targetRef.Name == "" {
			allErrs = append(allErrs, field.Required(targetPath.Child("name"), "targetRef name is required"))
		}
	}

	if policy.Spec.Timeout == nil && policy.Spec.Retry == nil && policy.Spec.RateLimit == nil {
		allErrs = append(allErrs, field.Required(field.NewPath("spec"), "at least one of timeout, retry, or rateLimit must be configured"))
	}

	if policy.Spec.Timeout != nil {
		if policy.Spec.Timeout.Duration == "" {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "timeout", "duration"), "duration is required"))
		} else if _, err := time.ParseDuration(policy.Spec.Timeout.Duration); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "timeout", "duration"), policy.Spec.Timeout.Duration, "must be a valid duration string"))
		}
	}

	if policy.Spec.Retry != nil && policy.Spec.Retry.Attempts <= 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "retry", "attempts"), policy.Spec.Retry.Attempts, "must be a positive integer"))
	}

	if policy.Spec.RateLimit != nil {
		if policy.Spec.RateLimit.RequestsPerUnit <= 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "rateLimit", "requestsPerUnit"), policy.Spec.RateLimit.RequestsPerUnit, "must be a positive integer"))
		}
		if _, ok := validRateLimitUnits[policy.Spec.RateLimit.Unit]; !ok {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "rateLimit", "unit"), policy.Spec.RateLimit.Unit, []string{"second", "minute", "hour"}))
		}
	}

	return allErrs
}

func ValidateTrafficPolicyUpdate(update, old *policyv1alpha1.TrafficPolicy) field.ErrorList {
	return ValidateTrafficPolicy(update)
}
