package status

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
)

func (w *Writer) updateRateLimitPolicy(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryState deliveryIndex,
	targets map[resourceKey]compiler.ResourceGeneration,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.RateLimitPolicies().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryState)
		targetStatuses := policyTargetStatuses(
			resource.Status.Targets,
			resource.Spec.TargetRefs,
			source,
			conditions,
			deliveryState,
			targets,
		)
		conditions = policyConditions(conditions, source, targetStatuses)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) &&
			equality.Semantic.DeepEqual(resource.Status.Targets, targetStatuses) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		updated.Status.Targets = targetStatuses
		_, err = w.client.RateLimitPolicies().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update RateLimitPolicy %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateIPRestrictionPolicy(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryState deliveryIndex,
	targets map[resourceKey]compiler.ResourceGeneration,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.IPRestrictionPolicies().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryState)
		targetStatuses := policyTargetStatuses(
			resource.Status.Targets,
			resource.Spec.TargetRefs,
			source,
			conditions,
			deliveryState,
			targets,
		)
		conditions = policyConditions(conditions, source, targetStatuses)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) &&
			equality.Semantic.DeepEqual(resource.Status.Targets, targetStatuses) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		updated.Status.Targets = targetStatuses
		_, err = w.client.IPRestrictionPolicies().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update IPRestrictionPolicy %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateHeaderTransformationPolicy(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryState deliveryIndex,
	targets map[resourceKey]compiler.ResourceGeneration,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.HeaderTransformationPolicies().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryState)
		targetStatuses := policyTargetStatuses(
			resource.Status.Targets,
			resource.Spec.TargetRefs,
			source,
			conditions,
			deliveryState,
			targets,
		)
		conditions = policyConditions(conditions, source, targetStatuses)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) &&
			equality.Semantic.DeepEqual(resource.Status.Targets, targetStatuses) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		updated.Status.Targets = targetStatuses
		_, err = w.client.HeaderTransformationPolicies().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update HeaderTransformationPolicy %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateMockResponsePolicy(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryState deliveryIndex,
	targets map[resourceKey]compiler.ResourceGeneration,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.MockResponsePolicies().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryState)
		targetStatuses := policyTargetStatuses(
			resource.Status.Targets,
			resource.Spec.TargetRefs,
			source,
			conditions,
			deliveryState,
			targets,
		)
		conditions = policyConditions(conditions, source, targetStatuses)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) &&
			equality.Semantic.DeepEqual(resource.Status.Targets, targetStatuses) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		updated.Status.Targets = targetStatuses
		_, err = w.client.MockResponsePolicies().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update MockResponsePolicy %q conditions: %w", source.Name, err)
	}
	return nil
}
