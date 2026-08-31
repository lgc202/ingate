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

func (w *Writer) updateGateway(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryState deliveryIndex,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Gateways().Get(ctx, source.Name, metav1.GetOptions{})
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
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.Gateways().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update Gateway %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateCertificate(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryState deliveryIndex,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Certificates().Get(ctx, source.Name, metav1.GetOptions{})
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
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.Certificates().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update Certificate %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateRoute(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryState deliveryIndex,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Routes().Get(ctx, source.Name, metav1.GetOptions{})
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
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.Routes().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update Route %q conditions: %w", source.Name, err)
	}
	return nil
}

func (w *Writer) updateUpstream(
	ctx context.Context,
	source compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryState deliveryIndex,
) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resource, err := w.client.Upstreams().Get(ctx, source.Name, metav1.GetOptions{})
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
		if equality.Semantic.DeepEqual(resource.Status.Conditions, conditions) {
			return nil
		}
		updated := resource.DeepCopy()
		updated.Status.Conditions = conditions
		_, err = w.client.Upstreams().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("update Upstream %q conditions: %w", source.Name, err)
	}
	return nil
}
