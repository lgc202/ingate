// Package dto 定义可由多个 Admin API 资源复用的产品响应模型
package dto

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
	messageInvalidSpec        = "配置内容不正确"
	messageReferenceNotFound  = "引用的资源不存在"
	messageInvalidReference   = "引用的资源不可用"
	messageConflict           = "配置与其他资源冲突"
	messageUnsupported        = "当前版本尚不支持该配置"
	messageCompileFailed      = "配置处理失败"
	messageRejected           = "配置未能生效"
	messageDeliveryFailed     = "配置发布失败"
)

// ResourceState 表示声明式资源面向控制台的处理状态
type ResourceState string

const (
	// ResourceStatePending 表示资源仍在等待控制面处理
	ResourceStatePending ResourceState = "Pending"
	// ResourceStateReady 表示当前资源版本已经生效
	ResourceStateReady ResourceState = "Ready"
	// ResourceStateError 表示当前资源版本无法生效
	ResourceStateError ResourceState = "Error"
	// ResourceStateDisabled 表示资源已被用户停用
	ResourceStateDisabled ResourceState = "Disabled"
)

// ResourceStatus 是控制台使用的声明式资源状态，不暴露底层 Condition 协议
type ResourceStatus struct {
	State   ResourceState `json:"state"`
	Message string        `json:"message"`
}

// NewResourceStatus 将当前 generation 的声明式 Condition 转换为产品状态
func NewResourceStatus(generation int64, conditions []metav1.Condition) ResourceStatus {
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
		return ResourceStatus{State: ResourceStatePending, Message: messagePending}
	}
	if hasResolvedRefs && (resolvedRefs == nil || resolvedRefs.Status != metav1.ConditionTrue) {
		return ResourceStatus{State: ResourceStatePending, Message: messageCheckingReferences}
	}
	if programmed == nil || programmed.Status != metav1.ConditionTrue {
		return ResourceStatus{State: ResourceStatePending, Message: messageProgramming}
	}
	return ResourceStatus{State: ResourceStateReady, Message: messageReady}
}

// NewDisabledResourceStatus 返回用户主动停用资源时的产品状态
func NewDisabledResourceStatus() ResourceStatus {
	return ResourceStatus{State: ResourceStateDisabled, Message: messageDisabled}
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

func errorStatus(condition *metav1.Condition) ResourceStatus {
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
	return ResourceStatus{State: ResourceStateError, Message: message}
}
