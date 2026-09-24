package validation

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateConditions(conditions []metav1.Condition, generation int64) field.ErrorList {
	path := field.NewPath("status", "conditions")
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
