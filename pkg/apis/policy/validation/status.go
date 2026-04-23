package validation

import (
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

func ValidateAuthPolicyStatusUpdate(update, old *policyv1alpha1.AuthPolicy) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateObservedGeneration(update.Status.ObservedGeneration, old.Generation, field.NewPath("status", "observedGeneration"))...)
	allErrs = append(allErrs, metav1validation.ValidateConditions(update.Status.Conditions, field.NewPath("status", "conditions"))...)

	return allErrs
}

func ValidateTrafficPolicyStatusUpdate(update, old *policyv1alpha1.TrafficPolicy) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateObservedGeneration(update.Status.ObservedGeneration, old.Generation, field.NewPath("status", "observedGeneration"))...)
	allErrs = append(allErrs, metav1validation.ValidateConditions(update.Status.Conditions, field.NewPath("status", "conditions"))...)

	return allErrs
}

func validateObservedGeneration(observedGeneration, currentGeneration int64, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if observedGeneration < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, observedGeneration, "must be zero or a positive integer"))
	}
	if currentGeneration > 0 && observedGeneration > currentGeneration {
		allErrs = append(allErrs, field.Invalid(fldPath, observedGeneration, "must not exceed metadata.generation"))
	}

	return allErrs
}
