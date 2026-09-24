package gateway

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

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

func (r *Reconciler) updateRouteStatuses(
	ctx context.Context,
	routes []gatewayv1alpha1.Route,
	rejections []routeRejection,
) error {
	byName := make(map[string]routeRejection, len(rejections))
	for _, rejection := range rejections {
		byName[rejection.route.Name] = rejection
	}

	for i := range routes {
		route := routes[i].DeepCopy()
		condition := acceptedCondition(route.Generation)
		if rejection, found := byName[route.Name]; found {
			condition.Status = metav1.ConditionFalse
			condition.Reason = string(rejection.reason)
			condition.Message = rejectionMessage(rejection)
		}

		if !meta.SetStatusCondition(&route.Status.Conditions, condition) {
			continue
		}
		if err := r.client.Status().Update(ctx, route); err != nil {
			return fmt.Errorf("update Route %q status: %w", route.Name, err)
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

func rejectionMessage(rejection routeRejection) string {
	switch rejection.reason {
	case gatewayMissing:
		return fmt.Sprintf("Gateway %q does not exist", rejection.relatedName)
	case gatewayRejected:
		return fmt.Sprintf("Gateway %q is not accepted", rejection.relatedName)
	case listenerMissing:
		return fmt.Sprintf("listener %q does not exist on the Gateway", rejection.relatedName)
	case upstreamMissing:
		return fmt.Sprintf("Upstream %q does not exist", rejection.relatedName)
	case matchConflict:
		return fmt.Sprintf("path match conflicts with Route %q", rejection.relatedName)
	default:
		return "Route was not accepted"
	}
}
