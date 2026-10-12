package validation

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/pkg/apis/gateway"
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

func validateRouteParentStatuses(parents []gateway.RouteParentStatus, generation int64) field.ErrorList {
	var errs field.ErrorList
	seen := make(map[string]bool)
	for i, parent := range parents {
		path := field.NewPath("status", "parents").Index(i)
		errs = append(errs, validateReferenceName(parent.ParentRef.Name, path.Child("parentRef", "name"))...)
		if parent.ParentRef.SectionName != "" {
			errs = append(errs, validateListenerName(parent.ParentRef.SectionName, path.Child("parentRef", "sectionName"))...)
		}
		if parent.ControllerName == "" {
			errs = append(errs, field.Required(path.Child("controllerName"), "is required"))
		} else {
			errs = append(errs, validation.IsDomainPrefixedPath(path.Child("controllerName"), parent.ControllerName)...)
		}
		key := parent.ControllerName + "/" + parent.ParentRef.Name + "/" + parent.ParentRef.SectionName
		if seen[key] {
			errs = append(errs, field.Duplicate(path, key))
		}
		seen[key] = true
		errs = append(errs, metav1validation.ValidateConditions(parent.Conditions, path.Child("conditions"))...)
		for j, condition := range parent.Conditions {
			if condition.ObservedGeneration > generation {
				errs = append(errs, field.Invalid(
					path.Child("conditions").Index(j).Child("observedGeneration"),
					condition.ObservedGeneration,
					"must not exceed metadata.generation",
				))
			}
		}
	}

	return errs
}
