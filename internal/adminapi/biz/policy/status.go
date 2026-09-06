package policy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Status 返回策略总体状态，停用配置只有进入 Active 后才显示为已停用。
func Status(generation int64, enabled bool, targetCount int, conditions []metav1.Condition) status.Status {
	if !enabled && status.IsApplied(generation, conditions) {
		return status.Status{State: status.StateDisabled, Reason: status.ReasonDisabled}
	}
	programmed := status.ProgrammedCondition(generation, conditions)
	if programmed != nil && programmed.Status == metav1.ConditionTrue {
		return status.Status{State: status.StateReady, Reason: status.ReasonReady}
	}
	if programmed != nil &&
		programmed.Status == metav1.ConditionFalse &&
		resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		if targetCount == 0 {
			return status.Status{State: status.StateReady, Reason: status.ReasonUnapplied}
		}
		return status.Status{State: status.StatePending, Reason: status.ReasonTargetNotApplied}
	}
	return status.FromConditions(generation, conditions)
}

// TargetStatus 返回指定策略目标的生效状态。
func TargetStatus(
	generation int64,
	disabled bool,
	ref resource.PolicyTargetRef,
	targets []resource.PolicyTargetStatus,
) status.Status {
	if disabled {
		return status.Status{State: status.StateDisabled, Reason: status.ReasonDisabled}
	}
	return targetStatus(generation, targetConditions(targets, ref))
}

func targetStatus(generation int64, conditions []metav1.Condition) status.Status {
	resolvedRefs, hasResolvedRefs := status.ResolvedRefsCondition(generation, conditions)
	programmed := status.ProgrammedCondition(generation, conditions)

	if resolvedRefs != nil && resolvedRefs.Status == metav1.ConditionFalse {
		return status.FromErrorCondition(resolvedRefs)
	}
	if programmed != nil &&
		programmed.Status == metav1.ConditionFalse &&
		resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		return status.Status{State: status.StatePending, Reason: status.ReasonTargetNotApplied}
	}
	if programmed != nil &&
		programmed.Status == metav1.ConditionFalse &&
		resource.ConditionReason(programmed.Reason) != resource.ReasonPending {
		return status.FromErrorCondition(programmed)
	}
	if hasResolvedRefs && (resolvedRefs == nil || resolvedRefs.Status != metav1.ConditionTrue) {
		return status.Status{State: status.StatePending, Reason: status.ReasonCheckingReferences}
	}
	if programmed == nil || programmed.Status != metav1.ConditionTrue {
		return status.Status{State: status.StatePending, Reason: status.ReasonProgramming}
	}
	return status.Status{State: status.StateReady, Reason: status.ReasonReady}
}

func targetConditions(targets []resource.PolicyTargetStatus, ref resource.PolicyTargetRef) []metav1.Condition {
	for _, target := range targets {
		if target.TargetRef.Kind == ref.Kind && target.TargetRef.Name == ref.Name {
			return target.Conditions
		}
	}
	return nil
}
