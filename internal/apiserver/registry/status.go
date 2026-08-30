package registry

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

// ValidateResourceStatus 校验普通资源的 Conditions 与当前 Generation 一致。
func ValidateResourceStatus(status resource.ResourceStatus, generation int64) field.ErrorList {
	return validateConditions(
		status.Conditions,
		generation,
		field.NewPath("status", "conditions"),
	)
}

// ValidatePolicyStatus 校验策略 Conditions 以及每个目标状态的归属关系。
func ValidatePolicyStatus(
	status resource.PolicyStatus,
	declaredTargets []resource.PolicyTargetRef,
	generation int64,
) field.ErrorList {
	errs := validateConditions(
		status.Conditions,
		generation,
		field.NewPath("status", "conditions"),
	)

	knownTargets := make(map[resource.PolicyTargetRef]bool, len(declaredTargets))
	for _, target := range declaredTargets {
		knownTargets[target] = true
	}
	seenTargets := make(map[resource.PolicyTargetRef]bool, len(status.Targets))
	targetsPath := field.NewPath("status", "targets")
	for i, targetStatus := range status.Targets {
		targetPath := targetsPath.Index(i)
		if !knownTargets[targetStatus.TargetRef] {
			errs = append(errs, field.NotFound(
				targetPath.Child("targetRef"),
				targetStatus.TargetRef,
			))
		}
		if seenTargets[targetStatus.TargetRef] {
			errs = append(errs, field.Duplicate(
				targetPath.Child("targetRef"),
				targetStatus.TargetRef,
			))
		}
		seenTargets[targetStatus.TargetRef] = true
		errs = append(errs, validateConditions(
			targetStatus.Conditions,
			generation,
			targetPath.Child("conditions"),
		)...)
	}
	return errs
}

func validateConditions(
	conditions []metav1.Condition,
	generation int64,
	path *field.Path,
) field.ErrorList {
	errs := metav1validation.ValidateConditions(conditions, path)
	for i, condition := range conditions {
		if condition.ObservedGeneration > generation {
			errs = append(errs, field.Invalid(
				path.Index(i).Child("observedGeneration"),
				condition.ObservedGeneration,
				"must not exceed metadata.generation",
			))
		}
	}
	return errs
}
