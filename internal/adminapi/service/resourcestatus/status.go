// Package resourcestatus 将声明式资源 Condition 转换为控制台使用的产品状态
package resourcestatus

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	messagePending            = "配置正在处理中"
	messageCheckingReferences = "正在检查关联资源"
	messageProgramming        = "配置正在生效"
	messageReady              = "配置已生效"
	messageDisabled           = "已停用"
	messageUnapplied          = "策略已保存，尚未应用"
	messageTargetNotApplied   = "目标当前没有可生效的流量入口"
	messageInvalidSpec        = "配置内容不正确"
	messageReferenceNotFound  = "引用的资源不存在"
	messageInvalidReference   = "引用的资源不可用"
	messageConflict           = "配置与其他资源冲突"
	messageUnsupported        = "当前版本尚不支持该配置"
	messageCompileFailed      = "配置处理失败"
	messageRejected           = "配置未能生效"
	messageDeliveryFailed     = "配置发布失败"
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

// Status 是控制台使用的声明式资源状态，不暴露底层 Condition 协议
type Status struct {
	State   State
	Message string
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
		return Status{State: StatePending, Message: messagePending}
	}
	if hasResolvedRefs && (resolvedRefs == nil || resolvedRefs.Status != metav1.ConditionTrue) {
		return Status{State: StatePending, Message: messageCheckingReferences}
	}
	if programmed == nil || programmed.Status != metav1.ConditionTrue {
		return Status{State: StatePending, Message: messageProgramming}
	}
	return Status{State: StateReady, Message: messageReady}
}

// ForPolicy 将策略总体 Condition 转换为产品状态
func ForPolicy(generation int64, targetCount int, conditions []metav1.Condition) Status {
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)
	if programmed != nil && programmed.Status == metav1.ConditionTrue {
		return Status{State: StateReady, Message: messageReady}
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		if targetCount == 0 {
			return Status{State: StateReady, Message: messageUnapplied}
		}
		return Status{State: StatePending, Message: messageTargetNotApplied}
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
		return Status{State: StatePending, Message: messageTargetNotApplied}
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) != resource.ReasonPending {
		return errorStatus(programmed)
	}
	if hasResolvedRefs && (resolvedRefs == nil || resolvedRefs.Status != metav1.ConditionTrue) {
		return Status{State: StatePending, Message: messageCheckingReferences}
	}
	if programmed == nil || programmed.Status != metav1.ConditionTrue {
		return Status{State: StatePending, Message: messageProgramming}
	}
	return Status{State: StateReady, Message: messageReady}
}

// Disabled 返回用户主动停用资源时的产品状态
func Disabled() Status {
	return Status{State: StateDisabled, Message: messageDisabled}
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
	message := messageCompileFailed
	switch resource.ConditionReason(condition.Reason) {
	case resource.ReasonInvalidSpec:
		message = messageInvalidSpec
	case resource.ReasonReferenceNotFound:
		message = messageReferenceNotFound
	case resource.ReasonInvalidReference:
		message = messageInvalidReference
	case resource.ReasonConflict:
		message = messageConflict
	case resource.ReasonUnsupported:
		message = messageUnsupported
	case resource.ReasonCompileFailed:
		message = messageCompileFailed
	case resource.ReasonRejected:
		message = messageRejected
	case resource.ReasonDeliveryFailed:
		message = messageDeliveryFailed
	}
	return Status{State: StateError, Message: message}
}
