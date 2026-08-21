package status

import (
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// policyConditions 汇总各目标状态：任一目标生效即视为策略已生效，具体异常保留在 targets 中
func policyConditions(
	conditions []metav1.Condition,
	resource compiler.ResourceGeneration,
	targets []gatewayv1.PolicyTargetStatus,
) []metav1.Condition {
	accepted := currentCondition(conditions, gatewayv1.ConditionAccepted, resource.Generation)
	if accepted == nil || accepted.Status != metav1.ConditionTrue {
		return conditions
	}
	if len(targets) == 0 {
		programmed := currentCondition(conditions, gatewayv1.ConditionProgrammed, resource.Generation)
		if programmed == nil || programmed.Status != metav1.ConditionTrue {
			return conditions
		}
		meta.SetStatusCondition(&conditions, newCondition(
			gatewayv1.ConditionProgrammed,
			resource.Generation,
			conditionDecision{
				status:  metav1.ConditionFalse,
				reason:  gatewayv1.ReasonNotApplied,
				message: messageNotApplied,
			},
		))
		return conditions
	}

	var failure *metav1.Condition
	hasPending := false
	for _, target := range targets {
		programmed := currentCondition(target.Conditions, gatewayv1.ConditionProgrammed, resource.Generation)
		if programmed == nil || programmed.Status == metav1.ConditionUnknown {
			hasPending = true
			continue
		}
		if programmed.Status == metav1.ConditionTrue {
			meta.SetStatusCondition(&conditions, newCondition(
				gatewayv1.ConditionProgrammed,
				resource.Generation,
				conditionDecision{
					status:  metav1.ConditionTrue,
					reason:  gatewayv1.ReasonProgrammed,
					message: messageProgrammed,
				},
			))
			return conditions
		}
		if gatewayv1.ConditionReason(programmed.Reason) != gatewayv1.ReasonNotApplied && failure == nil {
			current := *programmed
			failure = &current
		}
	}

	decision := conditionDecision{
		status:  metav1.ConditionFalse,
		reason:  gatewayv1.ReasonNotApplied,
		message: messageNotApplied,
	}
	if failure != nil {
		decision = conditionDecision{
			status:  failure.Status,
			reason:  gatewayv1.ConditionReason(failure.Reason),
			message: failure.Message,
		}
	} else if hasPending {
		decision = pendingDecision()
	}
	meta.SetStatusCondition(&conditions, newCondition(
		gatewayv1.ConditionProgrammed,
		resource.Generation,
		decision,
	))
	return conditions
}

// policyTargetStatuses 按声明顺序为每个 targetRef 计算独立状态，避免无效目标阻塞其他目标
func policyTargetStatuses(
	existing []gatewayv1.PolicyTargetStatus,
	targetRefs []gatewayv1.PolicyTargetRef,
	resource compiler.ResourceGeneration,
	policyConditions []metav1.Condition,
	deliveryStatus delivery.Status,
	targets map[resourceKey]compiler.ResourceGeneration,
	programmedTargets map[compiler.CompiledPolicyTarget]bool,
) []gatewayv1.PolicyTargetStatus {
	result := make([]gatewayv1.PolicyTargetStatus, 0, len(targetRefs))
	for _, targetRef := range targetRefs {
		conditions := existingPolicyTargetConditions(existing, targetRef)
		resolved := conditionDecision{
			status:  metav1.ConditionTrue,
			reason:  gatewayv1.ReasonResolvedRefs,
			message: messageTargetResolved,
		}
		target, exists := targets[resourceKey{kind: targetRef.Kind, name: targetRef.Name}]
		if !exists {
			resolved = conditionDecision{
				status:  metav1.ConditionFalse,
				reason:  gatewayv1.ReasonReferenceNotFound,
				message: fmt.Sprintf("Policy target %s %q does not exist", targetRef.Kind, targetRef.Name),
			}
		}
		meta.SetStatusCondition(&conditions, newCondition(
			gatewayv1.ConditionResolvedRefs,
			resource.Generation,
			resolved,
		))
		meta.SetStatusCondition(&conditions, policyTargetProgrammedCondition(
			policyConditions,
			resolved,
			resource,
			target,
			deliveryStatus,
			programmedTargets[compiler.CompiledPolicyTarget{
				Policy: resource,
				Target: target,
			}],
		))
		result = append(result, gatewayv1.PolicyTargetStatus{
			TargetRef:  targetRef,
			Conditions: conditions,
		})
	}
	return result
}

func existingPolicyTargetConditions(
	existing []gatewayv1.PolicyTargetStatus,
	target gatewayv1.PolicyTargetRef,
) []metav1.Condition {
	for _, status := range existing {
		if status.TargetRef == target {
			return slices.Clone(status.Conditions)
		}
	}
	return nil
}

func policyTargetProgrammedCondition(
	policyConditions []metav1.Condition,
	resolved conditionDecision,
	resource compiler.ResourceGeneration,
	target compiler.ResourceGeneration,
	deliveryStatus delivery.Status,
	isProgrammedTarget bool,
) metav1.Condition {
	accepted := currentCondition(policyConditions, gatewayv1.ConditionAccepted, resource.Generation)
	if accepted == nil {
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, pendingDecision())
	}
	if accepted.Status != metav1.ConditionTrue {
		return conditionBlockedBy(gatewayv1.ConditionProgrammed, resource.Generation, accepted)
	}
	if resolved.status != metav1.ConditionTrue {
		blocking := newCondition(gatewayv1.ConditionResolvedRefs, resource.Generation, resolved)
		return conditionBlockedBy(gatewayv1.ConditionProgrammed, resource.Generation, &blocking)
	}
	if slices.Contains(deliveryStatus.ActiveResources, resource) &&
		slices.Contains(deliveryStatus.ActiveResources, target) {
		if !isProgrammedTarget {
			return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, conditionDecision{
				status:  metav1.ConditionFalse,
				reason:  gatewayv1.ReasonNotApplied,
				message: messageNotApplied,
			})
		}
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, conditionDecision{
			status:  metav1.ConditionTrue,
			reason:  gatewayv1.ReasonProgrammed,
			message: messageProgrammed,
		})
	}
	failedTarget := compiler.CompiledPolicyTarget{Policy: resource, Target: target}
	if deliveryStatus.LastFailure != nil &&
		slices.Contains(deliveryStatus.LastFailure.Resources, resource) &&
		slices.Contains(deliveryStatus.LastFailure.PolicyTargets, failedTarget) {
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

func newPolicyTargetIndex(resources compiler.Resources) map[resourceKey]compiler.ResourceGeneration {
	targets := make(map[resourceKey]compiler.ResourceGeneration, len(resources.Gateways)+len(resources.Routes))
	for _, resource := range resources.Generations() {
		if resource.Kind == gatewayv1.KindGateway || resource.Kind == gatewayv1.KindRoute {
			targets[resourceKey{kind: resource.Kind, name: resource.Name}] = resource
		}
	}
	return targets
}

func newProgrammedPolicyTargetIndex(
	targets []compiler.CompiledPolicyTarget,
) map[compiler.CompiledPolicyTarget]bool {
	result := make(map[compiler.CompiledPolicyTarget]bool, len(targets))
	for _, target := range targets {
		result[target] = true
	}
	return result
}
