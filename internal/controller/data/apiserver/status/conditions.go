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

// deliveryIndex 为一次状态收敛建立只读索引，避免逐个资源线性扫描发布结果。
type deliveryIndex struct {
	activeResources     map[compiler.ResourceGeneration]bool
	activePolicyTargets map[compiler.CompiledPolicyTarget]bool
	failedResources     map[compiler.ResourceGeneration]bool
	failedPolicyTargets map[compiler.CompiledPolicyTarget]bool
	failureReason       delivery.FailureReason
}

func newDeliveryIndex(status delivery.Status) deliveryIndex {
	deliveryState := deliveryIndex{
		activeResources:     make(map[compiler.ResourceGeneration]bool, len(status.ActiveResources)),
		activePolicyTargets: make(map[compiler.CompiledPolicyTarget]bool, len(status.ActivePolicyTargets)),
	}
	for _, resource := range status.ActiveResources {
		deliveryState.activeResources[resource] = true
	}
	for _, target := range status.ActivePolicyTargets {
		deliveryState.activePolicyTargets[target] = true
	}
	if status.LastFailure == nil {
		return deliveryState
	}

	deliveryState.failedResources = make(map[compiler.ResourceGeneration]bool, len(status.LastFailure.Resources))
	deliveryState.failedPolicyTargets = make(map[compiler.CompiledPolicyTarget]bool, len(status.LastFailure.PolicyTargets))
	for _, resource := range status.LastFailure.Resources {
		deliveryState.failedResources[resource] = true
	}
	for _, target := range status.LastFailure.PolicyTargets {
		deliveryState.failedPolicyTargets[target] = true
	}
	deliveryState.failureReason = status.LastFailure.Reason
	return deliveryState
}

// resourceConditions 在编译结果缺席时只刷新 Programmed，保留当前 Generation 的编译结论。
func resourceConditions(
	existing []metav1.Condition,
	resource compiler.ResourceGeneration,
	compile *compileDecision,
	deliveryState deliveryIndex,
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

	meta.SetStatusCondition(&conditions, programmedCondition(conditions, resource, deliveryState))
	return conditions
}

func programmedCondition(
	conditions []metav1.Condition,
	resource compiler.ResourceGeneration,
	deliveryState deliveryIndex,
) metav1.Condition {
	accepted := currentCondition(conditions, gatewayv1.ConditionAccepted, resource.Generation)
	if accepted == nil {
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, pendingDecision())
	}
	if accepted.Status != metav1.ConditionTrue {
		return conditionBlockedBy(resource.Generation, accepted)
	}

	if kindHasReferences(resource.Kind) {
		resolvedRefs := currentCondition(conditions, gatewayv1.ConditionResolvedRefs, resource.Generation)
		if resolvedRefs == nil {
			return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, pendingDecision())
		}
		if resolvedRefs.Status != metav1.ConditionTrue {
			return conditionBlockedBy(resource.Generation, resolvedRefs)
		}
	}

	if deliveryState.activeResources[resource] {
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, conditionDecision{
			status:  metav1.ConditionTrue,
			reason:  gatewayv1.ReasonProgrammed,
			message: messageProgrammed,
		})
	}
	if deliveryState.failedResources[resource] {
		reason := gatewayv1.ReasonDeliveryFailed
		message := messageDeliveryFailed
		if deliveryState.failureReason == delivery.FailureRejected {
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
	return newCondition(gatewayv1.ConditionProgrammed, generation, conditionDecision{
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
