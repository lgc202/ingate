package gateway

import (
	"context"
	"fmt"
	"slices"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const controllerName = "gateway.ingate.io/gateway-controller"

func (r *Reconciler) updateGatewayStatuses(
	ctx context.Context,
	gateways []gatewayv1alpha1.Gateway,
	conflicts []gatewayConflict,
) error {
	byName := make(map[string]gatewayConflict, len(conflicts))
	for _, conflict := range conflicts {
		byName[conflict.gateway.Name] = conflict
	}

	for i := range gateways {
		gateway := gateways[i].DeepCopy()
		condition := acceptedCondition(gateway.Generation)
		if conflict, found := byName[gateway.Name]; found {
			condition.Status = metav1.ConditionFalse
			condition.Reason = "ListenerConflict"
			condition.Message = fmt.Sprintf(
				"listener %q conflicts with Gateway %q listener %q",
				conflict.listenerName, conflict.owner.Name, conflict.ownerListenerName,
			)
		}

		if !meta.SetStatusCondition(&gateway.Status.Conditions, condition) {
			continue
		}
		if err := r.client.Status().Update(ctx, gateway); err != nil {
			return fmt.Errorf("update Gateway %q status: %w", gateway.Name, err)
		}
	}

	return nil
}

func (r *Reconciler) updateHTTPRouteStatuses(
	ctx context.Context,
	routes []gatewayv1alpha1.HTTPRoute,
	parents []parentResolution,
) error {
	byName := make(map[string][]parentResolution)
	for _, parent := range parents {
		byName[parent.routeName] = append(byName[parent.routeName], parent)
	}

	for i := range routes {
		route := routes[i].DeepCopy()
		desired := make([]gatewayv1alpha1.RouteParentStatus, 0, len(byName[route.Name]))
		for _, state := range byName[route.Name] {
			parent := gatewayv1alpha1.RouteParentStatus{
				ParentRef:      state.parentRef,
				ControllerName: controllerName,
			}
			for _, previous := range route.Status.Parents {
				if previous.ParentRef == state.parentRef && previous.ControllerName == controllerName {
					parent.Conditions = slices.Clone(previous.Conditions)
					break
				}
			}

			accepted := acceptedCondition(route.Generation)
			accepted.Reason = state.reason
			accepted.Message = state.message
			if !state.accepted {
				accepted.Status = metav1.ConditionFalse
			}
			meta.SetStatusCondition(&parent.Conditions, accepted)

			resolved := metav1.Condition{
				Type:               "ResolvedRefs",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: route.Generation,
				Reason:             "ResolvedRefs",
				Message:            "all Backend references resolve",
			}
			if state.missingBackend != "" {
				resolved.Status = metav1.ConditionFalse
				resolved.Reason = "BackendNotFound"
				resolved.Message = fmt.Sprintf("Backend %q does not exist", state.missingBackend)
			}
			meta.SetStatusCondition(&parent.Conditions, resolved)
			desired = append(desired, parent)
		}

		// 仅替换本控制器拥有的父状态，保留其他控制器的记录。
		for _, parent := range route.Status.Parents {
			if parent.ControllerName != controllerName {
				desired = append(desired, parent)
			}
		}
		if apiequality.Semantic.DeepEqual(route.Status.Parents, desired) {
			continue
		}

		route.Status.Parents = desired
		if err := r.client.Status().Update(ctx, route); err != nil {
			return fmt.Errorf("update HTTPRoute %q status: %w", route.Name, err)
		}
	}

	return nil
}

func acceptedCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               gatewayv1alpha1.ConditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: generation,
		Reason:             "Accepted",
		Message:            "accepted into the shared data plane configuration",
	}
}
