package biz

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	statusPriorityError = iota
	statusPriorityPending
	statusPriorityReady
	statusPriorityDisabled
	statusPriorityUnknown
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

// ResourceReason 表示资源进入当前状态的产品语义原因
type ResourceReason string

const (
	// ReasonAwaitingAcceptance 表示控制面尚未接受当前配置
	ReasonAwaitingAcceptance ResourceReason = "AwaitingAcceptance"
	// ReasonCheckingReferences 表示控制面正在检查关联资源
	ReasonCheckingReferences ResourceReason = "CheckingReferences"
	// ReasonProgramming 表示配置正在发布到数据面
	ReasonProgramming ResourceReason = "Programming"
	// ReasonReady 表示当前配置已经生效
	ReasonReady ResourceReason = "Ready"
	// ReasonDisabled 表示资源已被用户停用
	ReasonDisabled ResourceReason = "Disabled"
	// ReasonUnapplied 表示策略已保存但没有作用目标
	ReasonUnapplied ResourceReason = "Unapplied"
	// ReasonTargetNotApplied 表示目标当前没有可生效的流量入口
	ReasonTargetNotApplied ResourceReason = "TargetNotApplied"
	// ReasonInvalidSpec 表示配置内容不正确
	ReasonInvalidSpec ResourceReason = "InvalidSpec"
	// ReasonReferenceNotFound 表示引用的资源不存在
	ReasonReferenceNotFound ResourceReason = "ReferenceNotFound"
	// ReasonInvalidReference 表示引用的资源不可用
	ReasonInvalidReference ResourceReason = "InvalidReference"
	// ReasonConflict 表示配置与其他资源冲突
	ReasonConflict ResourceReason = "Conflict"
	// ReasonUnsupported 表示当前版本不支持该配置
	ReasonUnsupported ResourceReason = "Unsupported"
	// ReasonCompileFailed 表示配置编译失败
	ReasonCompileFailed ResourceReason = "CompileFailed"
	// ReasonRejected 表示数据面拒绝当前配置
	ReasonRejected ResourceReason = "Rejected"
	// ReasonDeliveryFailed 表示配置发布失败
	ReasonDeliveryFailed ResourceReason = "DeliveryFailed"
)

// ResourceStatus 是控制台用例使用的资源状态，不暴露底层 Condition 协议
type ResourceStatus struct {
	State  ResourceState
	Reason ResourceReason
}

// ResourceStatusFromConditions 将当前 generation 的声明式 Condition 转换为产品状态
func ResourceStatusFromConditions(generation int64, conditions []metav1.Condition) ResourceStatus {
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
		return ResourceStatus{State: ResourceStatePending, Reason: ReasonAwaitingAcceptance}
	}
	if hasResolvedRefs && (resolvedRefs == nil || resolvedRefs.Status != metav1.ConditionTrue) {
		return ResourceStatus{State: ResourceStatePending, Reason: ReasonCheckingReferences}
	}
	if programmed == nil || programmed.Status != metav1.ConditionTrue {
		return ResourceStatus{State: ResourceStatePending, Reason: ReasonProgramming}
	}
	return ResourceStatus{State: ResourceStateReady, Reason: ReasonReady}
}

// PolicyResourceStatus 将策略总体 Condition 转换为产品状态
func PolicyResourceStatus(generation int64, targetCount int, conditions []metav1.Condition) ResourceStatus {
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)
	if programmed != nil && programmed.Status == metav1.ConditionTrue {
		return ResourceStatus{State: ResourceStateReady, Reason: ReasonReady}
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		if targetCount == 0 {
			return ResourceStatus{State: ResourceStateReady, Reason: ReasonUnapplied}
		}
		return ResourceStatus{State: ResourceStatePending, Reason: ReasonTargetNotApplied}
	}
	return ResourceStatusFromConditions(generation, conditions)
}

// PolicyTargetResourceStatus 将策略单个作用目标的 Condition 转换为产品状态
func PolicyTargetResourceStatus(generation int64, conditions []metav1.Condition) ResourceStatus {
	resolvedRefs, hasResolvedRefs := conditionForGeneration(generation, conditions, resource.ConditionResolvedRefs)
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)

	if resolvedRefs != nil && resolvedRefs.Status == metav1.ConditionFalse {
		return errorStatus(resolvedRefs)
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) == resource.ReasonNotApplied {
		return ResourceStatus{State: ResourceStatePending, Reason: ReasonTargetNotApplied}
	}
	if programmed != nil && programmed.Status == metav1.ConditionFalse && resource.ConditionReason(programmed.Reason) != resource.ReasonPending {
		return errorStatus(programmed)
	}
	if hasResolvedRefs && (resolvedRefs == nil || resolvedRefs.Status != metav1.ConditionTrue) {
		return ResourceStatus{State: ResourceStatePending, Reason: ReasonCheckingReferences}
	}
	if programmed == nil || programmed.Status != metav1.ConditionTrue {
		return ResourceStatus{State: ResourceStatePending, Reason: ReasonProgramming}
	}
	return ResourceStatus{State: ResourceStateReady, Reason: ReasonReady}
}

// DisabledResourceStatus 返回用户主动停用资源时的产品状态
func DisabledResourceStatus() ResourceStatus {
	return ResourceStatus{State: ResourceStateDisabled, Reason: ReasonDisabled}
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

// EnabledResourceStatus 同时考虑资源开关和当前版本是否已被控制面处理
func EnabledResourceStatus(generation int64, enabled bool, conditions []metav1.Condition) ResourceStatus {
	if !enabled && ConfigurationApplied(generation, conditions) {
		return DisabledResourceStatus()
	}
	return ResourceStatusFromConditions(generation, conditions)
}

// PolicyStatus 返回策略总体状态，停用配置只有进入 Active 后才显示为已停用
func PolicyStatus(generation int64, enabled bool, targetCount int, conditions []metav1.Condition) ResourceStatus {
	if !enabled && ConfigurationApplied(generation, conditions) {
		return DisabledResourceStatus()
	}
	return PolicyResourceStatus(generation, targetCount, conditions)
}

// PolicyTargetStatus 返回指定策略目标的生效状态
func PolicyTargetStatus(
	generation int64,
	disabled bool,
	ref resource.PolicyTargetRef,
	targets []resource.PolicyTargetStatus,
) ResourceStatus {
	if disabled {
		return DisabledResourceStatus()
	}
	return PolicyTargetResourceStatus(generation, targetConditions(targets, ref))
}

// effectivePolicyStatus 以最严重的目标状态表示策略当前实际生效结果
func effectivePolicyStatus(
	generation int64,
	enabled bool,
	refs []resource.PolicyTargetRef,
	conditions []metav1.Condition,
	targets []resource.PolicyTargetStatus,
) ResourceStatus {
	status := PolicyStatus(generation, enabled, len(refs), conditions)
	disabled := status.State == ResourceStateDisabled
	for _, ref := range refs {
		targetStatus := PolicyTargetStatus(generation, disabled, ref, targets)
		if statusPriority(targetStatus.State) < statusPriority(status.State) {
			status = targetStatus
		}
	}
	return status
}

func targetConditions(targets []resource.PolicyTargetStatus, ref resource.PolicyTargetRef) []metav1.Condition {
	for _, target := range targets {
		if target.TargetRef.Kind == ref.Kind && target.TargetRef.Name == ref.Name {
			return target.Conditions
		}
	}
	return nil
}

func statusPriority(state ResourceState) int {
	switch state {
	case ResourceStateError:
		return statusPriorityError
	case ResourceStatePending:
		return statusPriorityPending
	case ResourceStateReady:
		return statusPriorityReady
	case ResourceStateDisabled:
		return statusPriorityDisabled
	default:
		return statusPriorityUnknown
	}
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
	return ResourceStatus{State: ResourceStateError, Reason: reason}
}
