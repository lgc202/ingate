package status

import (
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/policyconfig"
)

// policyConditions 汇总各目标状态：任一目标生效即视为策略已生效，
// 具体异常保留在 targets 中。
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

// policyTargetStatuses 按声明顺序为每个 targetRef 计算独立状态，避免无效目标阻塞其他目标。
func policyTargetStatuses(
	existing []gatewayv1.PolicyTargetStatus,
	targetRefs []gatewayv1.PolicyTargetRef,
	resource compiler.ResourceGeneration,
	policyConditions []metav1.Condition,
	deliveryState deliveryIndex,
	targets map[resourceKey]compiler.ResourceGeneration,
	allowedTargetKinds ...gatewayv1.Kind,
) []gatewayv1.PolicyTargetStatus {
	if len(targetRefs) > policyconfig.MaxTargets {
		targetRefs = targetRefs[:policyconfig.MaxTargets]
	}
	existingConditions := make(map[gatewayv1.PolicyTargetRef][]metav1.Condition, len(existing))
	for _, status := range existing {
		existingConditions[status.TargetRef] = status.Conditions
	}

	result := make([]gatewayv1.PolicyTargetStatus, 0, len(targetRefs))
	for _, targetRef := range targetRefs {
		conditions := slices.Clone(existingConditions[targetRef])
		resolved := conditionDecision{
			status:  metav1.ConditionTrue,
			reason:  gatewayv1.ReasonResolvedRefs,
			message: messageTargetResolved,
		}
		target, exists := targets[resourceKey{kind: targetRef.Kind, name: targetRef.Name}]
		switch {
		case !slices.Contains(allowedTargetKinds, targetRef.Kind):
			resolved = conditionDecision{
				status:  metav1.ConditionFalse,
				reason:  gatewayv1.ReasonUnsupported,
				message: fmt.Sprintf("Policy target kind %q is not supported", targetRef.Kind),
			}
		case !exists:
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
		compiledTarget := compiler.CompiledPolicyTarget{
			Policy: resource,
			Target: target,
		}
		meta.SetStatusCondition(&conditions, policyTargetProgrammedCondition(
			policyConditions,
			resolved,
			resource,
			target,
			deliveryState,
			deliveryState.activePolicyTargets[compiledTarget],
		))
		result = append(result, gatewayv1.PolicyTargetStatus{
			TargetRef:  targetRef,
			Conditions: conditions,
		})
	}
	return result
}

func policyTargetProgrammedCondition(
	policyConditions []metav1.Condition,
	resolved conditionDecision,
	resource compiler.ResourceGeneration,
	target compiler.ResourceGeneration,
	deliveryState deliveryIndex,
	isProgrammedTarget bool,
) metav1.Condition {
	accepted := currentCondition(policyConditions, gatewayv1.ConditionAccepted, resource.Generation)
	if accepted == nil {
		return newCondition(gatewayv1.ConditionProgrammed, resource.Generation, pendingDecision())
	}
	if accepted.Status != metav1.ConditionTrue {
		return conditionBlockedBy(resource.Generation, accepted)
	}
	if resolved.status != metav1.ConditionTrue {
		blocking := newCondition(gatewayv1.ConditionResolvedRefs, resource.Generation, resolved)
		return conditionBlockedBy(resource.Generation, &blocking)
	}
	if deliveryState.activeResources[resource] && deliveryState.activeResources[target] {
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
	if deliveryState.failedResources[resource] && deliveryState.failedPolicyTargets[failedTarget] {
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

func newPolicyTargetIndex(resources []compiler.ResourceGeneration) map[resourceKey]compiler.ResourceGeneration {
	targets := make(map[resourceKey]compiler.ResourceGeneration, len(resources))
	for _, resource := range resources {
		if resource.Kind == gatewayv1.KindGateway || resource.Kind == gatewayv1.KindRoute {
			targets[resourceKey{kind: resource.Kind, name: resource.Name}] = resource
		}
	}
	return targets
}
