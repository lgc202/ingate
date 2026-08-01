// Package resourcestatus 将声明式资源 Condition 转换为控制台使用的产品状态
package resourcestatus

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// State 表示声明式资源面向控制台的处理状态
type State string

const (
	// StatePending 表示资源仍在等待控制面处理
	StatePending State = "Pending"
	// StateReady 表示当前资源版本已经生效
	StateReady State = "Ready"
	// StateError 表示当前资源版本无法生效
	StateError State = "Error"
	// StateDisabled 表示资源已被用户停用
	StateDisabled State = "Disabled"
)

// Reason 表示资源进入当前状态的产品语义原因
type Reason string

const (
	// ReasonAwaitingAcceptance 表示控制面尚未接受当前配置
	ReasonAwaitingAcceptance Reason = "AwaitingAcceptance"
	// ReasonCheckingReferences 表示控制面正在检查关联资源
	ReasonCheckingReferences Reason = "CheckingReferences"
	// ReasonProgramming 表示配置正在发布到数据面
	ReasonProgramming Reason = "Programming"
	// ReasonReady 表示当前配置已经生效
	ReasonReady Reason = "Ready"
	// ReasonDisabled 表示资源已被用户停用
	ReasonDisabled Reason = "Disabled"
	// ReasonUnapplied 表示策略已保存但没有作用目标
	ReasonUnapplied Reason = "Unapplied"
	// ReasonTargetNotApplied 表示目标当前没有可生效的流量入口
	ReasonTargetNotApplied Reason = "TargetNotApplied"
	// ReasonInvalidSpec 表示配置内容不正确
	ReasonInvalidSpec Reason = "InvalidSpec"
	// ReasonReferenceNotFound 表示引用的资源不存在
	ReasonReferenceNotFound Reason = "ReferenceNotFound"
	// ReasonInvalidReference 表示引用的资源不可用
	ReasonInvalidReference Reason = "InvalidReference"
	// ReasonConflict 表示配置与其他资源冲突
	ReasonConflict Reason = "Conflict"
	// ReasonUnsupported 表示当前版本不支持该配置
	ReasonUnsupported Reason = "Unsupported"
	// ReasonCompileFailed 表示配置编译失败
	ReasonCompileFailed Reason = "CompileFailed"
	// ReasonRejected 表示数据面拒绝当前配置
	ReasonRejected Reason = "Rejected"
	// ReasonDeliveryFailed 表示配置发布失败
	ReasonDeliveryFailed Reason = "DeliveryFailed"
)

// Status 是控制台用例使用的资源状态，不暴露底层 Condition 协议
type Status struct {
	State  State
	Reason Reason
}

// FromConditions 将当前 generation 的声明式 Condition 转换为产品状态
func FromConditions(generation int64, conditions []metav1.Condition) Status {
	accepted := currentCondition(generation, conditions, resource.ConditionAccepted)
	resolvedRefs, hasResolvedRefs := conditionForGeneration(generation, conditions, resource.ConditionResolvedRefs)
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)

	if accepted != nil && accepted.Status == metav1.ConditionFalse {
		return errorStatus(accepted)
	}
	if hasResolvedRefs && resolvedRefs != nil && resolvedRefs.Status == metav1.ConditionFalse {
		return errorStatus(resolvedRefs)
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) != resource.ReasonPending {
		return errorStatus(programmed)
	}

	if accepted == nil || accepted.Status != metav1.ConditionTrue {
		return Status{State: StatePending, Reason: ReasonAwaitingAcceptance}
	}
	if hasResolvedRefs && (resolvedRefs == nil || resolvedRefs.Status != metav1.ConditionTrue) {
		return Status{State: StatePending, Reason: ReasonCheckingReferences}
	}
	if programmed == nil || programmed.Status != metav1.ConditionTrue {
		return Status{State: StatePending, Reason: ReasonProgramming}
	}
	return Status{State: StateReady, Reason: ReasonReady}
}

// ForPolicy 将策略总体 Condition 转换为产品状态
func ForPolicy(generation int64, targetCount int, conditions []metav1.Condition) Status {
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)
	if programmed != nil && programmed.Status == metav1.ConditionTrue {
		return Status{State: StateReady, Reason: ReasonReady}
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		if targetCount == 0 {
			return Status{State: StateReady, Reason: ReasonUnapplied}
		}
		return Status{State: StatePending, Reason: ReasonTargetNotApplied}
	}
	return FromConditions(generation, conditions)
}

// ForPolicyTarget 将策略单个作用目标的 Condition 转换为产品状态
func ForPolicyTarget(generation int64, conditions []metav1.Condition) Status {
	resolvedRefs, hasResolvedRefs := conditionForGeneration(generation, conditions, resource.ConditionResolvedRefs)
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)

	if resolvedRefs != nil && resolvedRefs.Status == metav1.ConditionFalse {
		return errorStatus(resolvedRefs)
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		return Status{State: StatePending, Reason: ReasonTargetNotApplied}
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) != resource.ReasonPending {
		return errorStatus(programmed)
	}
	if hasResolvedRefs && (resolvedRefs == nil || resolvedRefs.Status != metav1.ConditionTrue) {
		return Status{State: StatePending, Reason: ReasonCheckingReferences}
	}
	if programmed == nil || programmed.Status != metav1.ConditionTrue {
		return Status{State: StatePending, Reason: ReasonProgramming}
	}
	return Status{State: StateReady, Reason: ReasonReady}
}

// Disabled 返回用户主动停用资源时的产品状态
func Disabled() Status {
	return Status{State: StateDisabled, Reason: ReasonDisabled}
}

// ConfigurationApplied 判断当前 generation 的停用或未应用结果是否已经进入 Active 配置
func ConfigurationApplied(generation int64, conditions []metav1.Condition) bool {
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

func conditionForGeneration(generation int64, conditions []metav1.Condition, conditionType resource.ConditionType) (*metav1.Condition, bool) {
	value := apimeta.FindStatusCondition(conditions, string(conditionType))
	if value == nil {
		return nil, false
	}
	if value.ObservedGeneration != generation {
		return nil, true
	}
	return value, true
}

func currentCondition(generation int64, conditions []metav1.Condition, conditionType resource.ConditionType) *metav1.Condition {
	value, _ := conditionForGeneration(generation, conditions, conditionType)
	return value
}

func errorStatus(condition *metav1.Condition) Status {
	reason := ReasonCompileFailed
	switch resource.ConditionReason(condition.Reason) {
	case resource.ReasonInvalidSpec:
		reason = ReasonInvalidSpec
	case resource.ReasonReferenceNotFound:
		reason = ReasonReferenceNotFound
	case resource.ReasonInvalidReference:
		reason = ReasonInvalidReference
	case resource.ReasonConflict:
		reason = ReasonConflict
	case resource.ReasonUnsupported:
		reason = ReasonUnsupported
	case resource.ReasonCompileFailed:
		reason = ReasonCompileFailed
	case resource.ReasonRejected:
		reason = ReasonRejected
	case resource.ReasonDeliveryFailed:
		reason = ReasonDeliveryFailed
	}
	return Status{State: StateError, Reason: reason}
}
