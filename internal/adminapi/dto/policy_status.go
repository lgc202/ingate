package dto

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const messageUnapplied = "策略已保存，尚未应用"
const messageTargetNotApplied = "目标当前没有可生效的流量入口"

// PolicyStatus 是控制台使用的策略总体状态
type PolicyStatus struct {
	State   ResourceState `json:"state"`
	Message string        `json:"message"`
}

// NewPolicyStatus 将策略 Condition 转换为产品状态
func NewPolicyStatus(generation int64, targetCount int, conditions []metav1.Condition) PolicyStatus {
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)
	if programmed != nil && programmed.Status == metav1.ConditionTrue {
		return PolicyStatus{State: ResourceStateReady, Message: messageReady}
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		if targetCount == 0 {
			return PolicyStatus{State: ResourceStateReady, Message: messageUnapplied}
		}
		return PolicyStatus{State: ResourceStatePending, Message: messageTargetNotApplied}
	}

	status := NewResourceStatus(generation, conditions)
	return PolicyStatus{State: status.State, Message: status.Message}
}

// NewPolicyTargetStatus 将策略单个作用目标的 Condition 转换为产品状态
func NewPolicyTargetStatus(generation int64, conditions []metav1.Condition) ResourceStatus {
	resolvedRefs, hasResolvedRefs := conditionForGeneration(generation, conditions, resource.ConditionResolvedRefs)
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)

	if resolvedRefs != nil && resolvedRefs.Status == metav1.ConditionFalse {
		return errorStatus(resolvedRefs)
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		return ResourceStatus{State: ResourceStatePending, Message: messageTargetNotApplied}
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) != resource.ReasonPending {
		return errorStatus(programmed)
	}
	if hasResolvedRefs && (resolvedRefs == nil || resolvedRefs.Status != metav1.ConditionTrue) {
		return ResourceStatus{State: ResourceStatePending, Message: messageCheckingReferences}
	}
	if programmed == nil || programmed.Status != metav1.ConditionTrue {
		return ResourceStatus{State: ResourceStatePending, Message: messageProgramming}
	}
	return ResourceStatus{State: ResourceStateReady, Message: messageReady}
}

// NewDisabledPolicyStatus 返回用户主动停用策略时的产品状态
func NewDisabledPolicyStatus() PolicyStatus {
	return PolicyStatus{State: ResourceStateDisabled, Message: messageDisabled}
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
