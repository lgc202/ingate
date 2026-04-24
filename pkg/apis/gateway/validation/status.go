package validation

import (
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func ValidateGatewayStatusUpdate(update, old *gatewayv1alpha1.Gateway) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateObservedGeneration(update.Status.ObservedGeneration, old.Generation, field.NewPath("status", "observedGeneration"))...)
	allErrs = append(allErrs, metav1validation.ValidateConditions(update.Status.Conditions, field.NewPath("status", "conditions"))...)

	seenNames := map[string]struct{}{}
	for i, listener := range update.Status.Listeners {
		listenerPath := field.NewPath("status", "listeners").Index(i)
		if listener.Name == "" {
			allErrs = append(allErrs, field.Required(listenerPath.Child("name"), "listener status name is required"))
		} else {
			if _, exists := seenNames[listener.Name]; exists {
				allErrs = append(allErrs, field.Duplicate(listenerPath.Child("name"), listener.Name))
			}
			seenNames[listener.Name] = struct{}{}
		}
		if listener.AttachedRoutes < 0 {
			allErrs = append(allErrs, field.Invalid(listenerPath.Child("attachedRoutes"), listener.AttachedRoutes, "must be zero or a positive integer"))
		}
		allErrs = append(allErrs, metav1validation.ValidateConditions(listener.Conditions, listenerPath.Child("conditions"))...)
	}

	return allErrs
}

func ValidateRouteStatusUpdate(update, old *gatewayv1alpha1.Route) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateObservedGeneration(update.Status.ObservedGeneration, old.Generation, field.NewPath("status", "observedGeneration"))...)
	allErrs = append(allErrs, metav1validation.ValidateConditions(update.Status.Conditions, field.NewPath("status", "conditions"))...)

	seenNames := map[string]struct{}{}
	for i, parent := range update.Status.Parents {
		parentPath := field.NewPath("status", "parents").Index(i)
		if parent.Name == "" {
			allErrs = append(allErrs, field.Required(parentPath.Child("name"), "parent status name is required"))
		} else {
			if _, exists := seenNames[parent.Name]; exists {
				allErrs = append(allErrs, field.Duplicate(parentPath.Child("name"), parent.Name))
			}
			seenNames[parent.Name] = struct{}{}
		}
		allErrs = append(allErrs, metav1validation.ValidateConditions(parent.Conditions, parentPath.Child("conditions"))...)
	}

	return allErrs
}

func ValidateBackendStatusUpdate(update, old *gatewayv1alpha1.Backend) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateObservedGeneration(update.Status.ObservedGeneration, old.Generation, field.NewPath("status", "observedGeneration"))...)
	allErrs = append(allErrs, metav1validation.ValidateConditions(update.Status.Conditions, field.NewPath("status", "conditions"))...)

	for i, endpoint := range update.Status.Endpoints {
		allErrs = append(allErrs, validateEndpoint(endpoint, field.NewPath("status", "endpoints").Index(i))...)
	}

	return allErrs
}

func ValidateCertificateStatusUpdate(update, old *gatewayv1alpha1.Certificate) field.ErrorList {
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
