package gateway

import (
	"context"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type gatewayConflict struct {
	gateway           *gatewayv1alpha1.Gateway
	listenerName      string
	owner             *gatewayv1alpha1.Gateway
	ownerListenerName string
}

type listenerBinding struct {
	gateway  *gatewayv1alpha1.Gateway
	listener gatewayv1alpha1.Listener
}

func (r *resolution) selectGateways(gateways []gatewayv1alpha1.Gateway) {
	ordered := make([]*gatewayv1alpha1.Gateway, 0, len(gateways))
	for i := range gateways {
		if !gateways[i].DeletionTimestamp.IsZero() {
			continue
		}

		ordered = append(ordered, &gateways[i])
	}

	slices.SortFunc(ordered, func(a, b *gatewayv1alpha1.Gateway) int {
		return compareMeta(a.ObjectMeta, b.ObjectMeta)
	})

	var bindings []listenerBinding
	for _, candidate := range ordered {
		if conflict, found := findListenerConflict(candidate, bindings); found {
			r.gatewayConflicts = append(r.gatewayConflicts, conflict)
			continue
		}

		r.gateways = append(r.gateways, candidate)
		for _, listener := range candidate.Spec.Listeners {
			bindings = append(bindings, listenerBinding{gateway: candidate, listener: listener})
		}
	}
}

func (r *Reconciler) updateGatewayStatuses(
	ctx context.Context,
	gateways []gatewayv1alpha1.Gateway,
	conflicts []gatewayConflict,
) error {
	byKey := make(map[client.ObjectKey]gatewayConflict, len(conflicts))
	for _, conflict := range conflicts {
		byKey[client.ObjectKeyFromObject(conflict.gateway)] = conflict
	}

	for i := range gateways {
		compiled := &gateways[i]
		key := client.ObjectKeyFromObject(compiled)
		var gateway gatewayv1alpha1.Gateway
		if err := r.client.Get(ctx, key, &gateway); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			return fmt.Errorf("get Gateway %q for status: %w", key.String(), err)
		}

		// 发布的是派生版本，不能替尚未编译的新声明确认 ObservedGeneration。
		if gateway.UID != compiled.UID || gateway.Generation != compiled.Generation {
			continue
		}

		condition := acceptedCondition(gateway.Generation)
		if conflict, found := byKey[key]; found {
			condition.Status = metav1.ConditionFalse
			condition.Reason = "ListenerConflict"
			condition.Message = fmt.Sprintf(
				"listener %q conflicts with Gateway %q listener %q",
				conflict.listenerName, client.ObjectKeyFromObject(conflict.owner).String(), conflict.ownerListenerName,
			)
		}

		if !meta.SetStatusCondition(&gateway.Status.Conditions, condition) {
			continue
		}

		if err := r.client.Status().Update(ctx, &gateway); err != nil {
			return fmt.Errorf("update Gateway %q status: %w", key.String(), err)
		}
	}

	return nil
}

func findListenerConflict(candidate *gatewayv1alpha1.Gateway, bindings []listenerBinding) (gatewayConflict, bool) {
	for _, listener := range candidate.Spec.Listeners {
		for _, binding := range bindings {
			if listener.Port != binding.listener.Port {
				continue
			}

			if listener.Hostname != "" && binding.listener.Hostname != "" && listener.Hostname != binding.listener.Hostname {
				continue
			}

			return gatewayConflict{
				gateway:           candidate,
				listenerName:      listener.Name,
				owner:             binding.gateway,
				ownerListenerName: binding.listener.Name,
			}, true
		}
	}

	return gatewayConflict{}, false
}
