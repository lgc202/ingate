package status

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/controller/compiler"
	"github.com/lgc202/ingate/internal/controller/delivery"
)

func (w *Writer) updateRateLimitPolicy(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
	targets map[resourceKey]compiler.ResourceGeneration,
	programmedTargets map[compiler.CompiledPolicyTarget]bool,
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

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryStatus)
		targetStatuses := policyTargetStatuses(
			resource.Status.Targets,
			resource.Spec.TargetRefs,
			source,
			conditions,
			deliveryStatus,
			targets,
			programmedTargets,
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

func (w *Writer) updateAccessControlPolicy(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
	targets map[resourceKey]compiler.ResourceGeneration,
	programmedTargets map[compiler.CompiledPolicyTarget]bool,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.AccessControlPolicies().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryStatus)
		targetStatuses := policyTargetStatuses(
			resource.Status.Targets,
			resource.Spec.TargetRefs,
			source,
			conditions,
			deliveryStatus,
			targets,
			programmedTargets,
		)
		conditions = policyConditions(conditions, source, targetStatuses)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) &&
			equality.Semantic.DeepEqual(resource.Status.Targets, targetStatuses) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		updated.Status.Targets = targetStatuses
		_, err = w.client.AccessControlPolicies().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update AccessControlPolicy %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateTokenQuotaPolicy(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
	targets map[resourceKey]compiler.ResourceGeneration,
	programmedTargets map[compiler.CompiledPolicyTarget]bool,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.TokenQuotaPolicies().Get(ctx, source.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.UID != source.UID || resource.Generation != source.Generation {
			return nil
		}

		conditions := resourceConditions(resource.Status.Conditions, source, compile, deliveryStatus)
		targetStatuses := policyTargetStatuses(
			resource.Status.Targets,
			resource.Spec.TargetRefs,
			source,
			conditions,
			deliveryStatus,
			targets,
			programmedTargets,
		)
		conditions = policyConditions(conditions, source, targetStatuses)
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) &&
			equality.Semantic.DeepEqual(resource.Status.Targets, targetStatuses) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		updated.Status.Targets = targetStatuses
		_, err = w.client.TokenQuotaPolicies().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update TokenQuotaPolicy %q conditions: %w", source.Name, err)
	}
	return nil
}
