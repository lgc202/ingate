package status

import (
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

const (
	messageAccepted       = "Resource is accepted"
	messageResolvedRefs   = "All resource references are resolved"
	messageProgrammed     = "Resource configuration is active"
	messagePending        = "Resource configuration is pending"
	messageRejected       = "Resource configuration was rejected"
	messageDeliveryFailed = "Resource configuration could not be applied"
	messageCompileFailed  = "Resource configuration could not be compiled"
	messageNotApplied     = "Policy is not applied to any target"
	messageTargetResolved = "Policy target is resolved"
)

type conditionDecision struct {
	status  metav1.ConditionStatus
	reason  gatewayv1.ConditionReason
	message string
}

type compileDecision struct {
	accepted     conditionDecision
	resolvedRefs *conditionDecision
}

// resourceConditions 在编译结果缺席时只刷新 Programmed，保留当前 Generation 的编译结论
func resourceConditions(
	existing []metav1.Condition,
	resource compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryStatus delivery.Status,
) []metav1.Condition {
	conditions := slices.Clone(existing)
	if !kindHasReferences(resource.Kind) {
		meta.RemoveStatusCondition(&conditions, string(gatewayv1.ConditionResolvedRefs))
	}
	if compile != nil {
		meta.SetStatusCondition(&conditions, newCondition(
			gatewayv1.ConditionAccepted,
			resource.Generation,
			compile.accepted,
		))
		if compile.resolvedRefs != nil {
			meta.SetStatusCondition(&conditions, newCondition(
				gatewayv1.ConditionResolvedRefs,
				resource.Generation,
				*compile.resolvedRefs,
			))
		}
	}

	meta.SetStatusCondition(&conditions, programmedCondition(conditions, resource, deliveryStatus))
	return conditions
}

func programmedCondition(
	conditions []metav1.Condition,
	resource compiler.ResourceGeneration,
	deliveryStatus delivery.Status,
) metav1.Condition {
	accepted := currentCondition(conditions, gatewayv1.ConditionAccepted, resource.Generation)
	if accepted == nil {
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, pendingDecision())
	}
	if accepted.Status != metav1.ConditionTrue {
		return conditionBlockedBy(gatewayv1.ConditionProgrammed, resource.Generation, accepted)
	}

	if kindHasReferences(resource.Kind) {
		resolvedRefs := currentCondition(conditions, gatewayv1.ConditionResolvedRefs, resource.Generation)
		if resolvedRefs == nil {
			return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, pendingDecision())
		}
		if resolvedRefs.Status != metav1.ConditionTrue {
			return conditionBlockedBy(gatewayv1.ConditionProgrammed, resource.Generation, resolvedRefs)
		}
	}

	if slices.Contains(deliveryStatus.ActiveResources, resource) {
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, conditionDecision{
			status:  metav1.ConditionTrue,
			reason:  gatewayv1.ReasonProgrammed,
			message: messageProgrammed,
		})
	}
	if deliveryStatus.LastFailure != nil && slices.Contains(deliveryStatus.LastFailure.Resources, resource) {
		reason := gatewayv1.ReasonDeliveryFailed
		message := messageDeliveryFailed
		if deliveryStatus.LastFailure.Reason == delivery.FailureRejected {
			reason = gatewayv1.ReasonRejected
			message = messageRejected
		}
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, conditionDecision{
			status:  metav1.ConditionFalse,
			reason:  reason,
			message: message,
		})
	}
	return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, pendingDecision())
}

func conditionBlockedBy(
	conditionType gatewayv1.ConditionType,
	generation int64,
	blocking *metav1.Condition,
) metav1.Condition {
	status := metav1.ConditionUnknown
	reason := gatewayv1.ReasonPending
	message := messagePending
	if blocking.Status == metav1.ConditionFalse {
		status = metav1.ConditionFalse
		reason = gatewayv1.ConditionReason(blocking.Reason)
		message = blocking.Message
	}
	return newCondition(conditionType, generation, conditionDecision{
		status:  status,
		reason:  reason,
		message: message,
	})
}

func currentCondition(
	conditions []metav1.Condition,
	conditionType gatewayv1.ConditionType,
	generation int64,
) *metav1.Condition {
	condition := meta.FindStatusCondition(conditions, string(conditionType))
	if condition == nil || condition.ObservedGeneration != generation {
		return nil
	}
	return condition
}

func newCondition(
	conditionType gatewayv1.ConditionType,
	generation int64,
	decision conditionDecision,
) metav1.Condition {
	return metav1.Condition{
		Type:               string(conditionType),
		Status:             decision.status,
		ObservedGeneration: generation,
		Reason:             string(decision.reason),
		Message:            decision.message,
	}
}

func pendingDecision() conditionDecision {
	return conditionDecision{
		status:  metav1.ConditionUnknown,
		reason:  gatewayv1.ReasonPending,
		message: messagePending,
	}
}

func kindHasReferences(kind gatewayv1.Kind) bool {
	switch kind {
	case gatewayv1.KindGateway,
		gatewayv1.KindRoute,
		gatewayv1.KindRateLimitPolicy,
		gatewayv1.KindIPRestrictionPolicy,
		gatewayv1.KindHeaderTransformationPolicy,
		gatewayv1.KindMockResponsePolicy:
		return true
	default:
		return false
	}
}
