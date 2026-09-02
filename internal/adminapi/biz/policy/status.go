package policy

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Status 返回策略总体状态，停用配置只有进入 Active 后才显示为已停用。
func Status(generation int64, enabled bool, targetCount int, conditions []metav1.Condition) resourceview.Status {
	if !enabled && configurationApplied(generation, conditions) {
		return resourceview.Status{State: resourceview.StateDisabled, Reason: resourceview.ReasonDisabled}
	}
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)
	if programmed != nil && programmed.Status == metav1.ConditionTrue {
		return resourceview.Status{State: resourceview.StateReady, Reason: resourceview.ReasonReady}
	}
	if programmed != nil &&
		programmed.Status == metav1.ConditionFalse &&
		resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		if targetCount == 0 {
			return resourceview.Status{State: resourceview.StateReady, Reason: resourceview.ReasonUnapplied}
		}
		return resourceview.Status{State: resourceview.StatePending, Reason: resourceview.ReasonTargetNotApplied}
	}
	return resourceview.StatusFromConditions(generation, conditions)
}

// TargetStatus 返回指定策略目标的生效状态。
func TargetStatus(
	generation int64,
	disabled bool,
	ref resource.PolicyTargetRef,
	targets []resource.PolicyTargetStatus,
) resourceview.Status {
	if disabled {
		return resourceview.Status{State: resourceview.StateDisabled, Reason: resourceview.ReasonDisabled}
	}
	return targetStatus(generation, targetConditions(targets, ref))
}

func targetStatus(generation int64, conditions []metav1.Condition) resourceview.Status {
	resolvedRefs, hasResolvedRefs := conditionForGeneration(
		generation,
		conditions,
		resource.ConditionResolvedRefs,
	)
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)

	if resolvedRefs != nil && resolvedRefs.Status == metav1.ConditionFalse {
		return resourceview.ErrorStatus(resolvedRefs)
	}
	if programmed != nil &&
		programmed.Status == metav1.ConditionFalse &&
		resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		return resourceview.Status{State: resourceview.StatePending, Reason: resourceview.ReasonTargetNotApplied}
	}
	if programmed != nil &&
		programmed.Status == metav1.ConditionFalse &&
		resource.ConditionReason(programmed.Reason) != resource.ReasonPending {
		return resourceview.ErrorStatus(programmed)
	}
	if hasResolvedRefs && (resolvedRefs == nil || resolvedRefs.Status != metav1.ConditionTrue) {
		return resourceview.Status{State: resourceview.StatePending, Reason: resourceview.ReasonCheckingReferences}
	}
	if programmed == nil || programmed.Status != metav1.ConditionTrue {
		return resourceview.Status{State: resourceview.StatePending, Reason: resourceview.ReasonProgramming}
	}
	return resourceview.Status{State: resourceview.StateReady, Reason: resourceview.ReasonReady}
}

func configurationApplied(generation int64, conditions []metav1.Condition) bool {
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)
	if programmed == nil {
		return false
	}
	if programmed.Status == metav1.ConditionTrue {
		return true
	}
	return programmed.Status == metav1.ConditionFalse &&
		resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied
}

func targetConditions(targets []resource.PolicyTargetStatus, ref resource.PolicyTargetRef) []metav1.Condition {
	for _, target := range targets {
		if target.TargetRef.Kind == ref.Kind && target.TargetRef.Name == ref.Name {
			return target.Conditions
		}
	}
	return nil
}

func conditionForGeneration(
	generation int64,
	conditions []metav1.Condition,
	conditionType resource.ConditionType,
) (*metav1.Condition, bool) {
	value := apimeta.FindStatusCondition(conditions, string(conditionType))
	if value == nil {
		return nil, false
	}
	if value.ObservedGeneration != generation {
		return nil, true
	}
	return value, true
}

func currentCondition(
	generation int64,
	conditions []metav1.Condition,
	conditionType resource.ConditionType,
) *metav1.Condition {
	value, _ := conditionForGeneration(generation, conditions, conditionType)
	return value
}
