package resourceview

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// State 表示声明式资源面向控制台的处理状态。
type State string

const (
	// StatePending 表示资源仍在等待控制面处理。
	StatePending State = "Pending"
	// StateReady 表示当前资源版本已经生效。
	StateReady State = "Ready"
	// StateError 表示当前资源版本无法生效。
	StateError State = "Error"
	// StateDisabled 表示资源已被用户停用。
	StateDisabled State = "Disabled"
)

// Reason 表示资源进入当前状态的产品语义原因。
type Reason string

const (
	// ReasonAwaitingAcceptance 表示控制面尚未接受当前配置。
	ReasonAwaitingAcceptance Reason = "AwaitingAcceptance"
	// ReasonCheckingReferences 表示控制面正在检查关联资源。
	ReasonCheckingReferences Reason = "CheckingReferences"
	// ReasonProgramming 表示配置正在发布到数据面。
	ReasonProgramming Reason = "Programming"
	// ReasonReady 表示当前配置已经生效。
	ReasonReady Reason = "Ready"
	// ReasonDisabled 表示资源已被用户停用。
	ReasonDisabled Reason = "Disabled"
	// ReasonUnapplied 表示配置已保存但没有作用目标。
	ReasonUnapplied Reason = "Unapplied"
	// ReasonTargetNotApplied 表示目标当前没有可生效的流量入口。
	ReasonTargetNotApplied Reason = "TargetNotApplied"
	// ReasonInvalidSpec 表示配置内容不正确。
	ReasonInvalidSpec Reason = "InvalidSpec"
	// ReasonReferenceNotFound 表示引用的资源不存在。
	ReasonReferenceNotFound Reason = "ReferenceNotFound"
	// ReasonPluginNotInstalled 表示策略依赖的数据面插件尚未安装。
	ReasonPluginNotInstalled Reason = "PluginNotInstalled"
	// ReasonInvalidReference 表示引用的资源不可用。
	ReasonInvalidReference Reason = "InvalidReference"
	// ReasonConflict 表示配置与其他资源冲突。
	ReasonConflict Reason = "Conflict"
	// ReasonUnsupported 表示当前版本不支持该配置。
	ReasonUnsupported Reason = "Unsupported"
	// ReasonCompileFailed 表示配置编译失败。
	ReasonCompileFailed Reason = "CompileFailed"
	// ReasonArtifactUnavailable 表示插件制品无法下载或校验。
	ReasonArtifactUnavailable Reason = "ArtifactUnavailable"
	// ReasonRejected 表示数据面拒绝当前配置。
	ReasonRejected Reason = "Rejected"
	// ReasonDeliveryFailed 表示配置发布失败。
	ReasonDeliveryFailed Reason = "DeliveryFailed"
)

// Status 是控制台用例使用的资源状态，不暴露底层 Condition 协议。
type Status struct {
	State  State
	Reason Reason
}

// StatusFromConditions 将当前 generation 的声明式 Condition 转换为产品状态。
func StatusFromConditions(generation int64, conditions []metav1.Condition) Status {
	accepted := currentCondition(generation, conditions, resource.ConditionAccepted)
	resolvedRefs, hasResolvedRefs := conditionForGeneration(generation, conditions, resource.ConditionResolvedRefs)
	programmed := currentCondition(generation, conditions, resource.ConditionProgrammed)

	if accepted != nil && accepted.Status == metav1.ConditionFalse {
		return ErrorStatus(accepted)
	}
	if hasResolvedRefs && resolvedRefs != nil && resolvedRefs.Status == metav1.ConditionFalse {
		return ErrorStatus(resolvedRefs)
	}
	if programmed != nil &&
		programmed.Status == metav1.ConditionFalse &&
		resource.ConditionReason(programmed.Reason) != resource.ReasonPending {
		return ErrorStatus(programmed)
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

// WasmPluginStatus 只根据插件制品的下载与校验结果判断安装状态。
// 插件是否作用到流量由引用它的强类型策略状态表达，
// 不与整套 Envoy 配置发布状态耦合。
func WasmPluginStatus(generation int64, conditions []metav1.Condition) Status {
	accepted := currentCondition(generation, conditions, resource.ConditionAccepted)
	if accepted != nil && accepted.Status == metav1.ConditionFalse {
		return ErrorStatus(accepted)
	}
	if accepted == nil || accepted.Status != metav1.ConditionTrue {
		return Status{State: StatePending, Reason: ReasonAwaitingAcceptance}
	}
	return Status{State: StateReady, Reason: ReasonReady}
}

// EnabledStatus 同时考虑资源开关和当前版本是否已被控制面处理。
func EnabledStatus(generation int64, enabled bool, conditions []metav1.Condition) Status {
	if !enabled && configurationApplied(generation, conditions) {
		return disabledStatus()
	}
	return StatusFromConditions(generation, conditions)
}

// ErrorStatus 将失败 Condition 转换为安全的产品状态。
func ErrorStatus(condition *metav1.Condition) Status {
	reason := ReasonCompileFailed
	switch resource.ConditionReason(condition.Reason) {
	case resource.ReasonInvalidSpec:
		reason = ReasonInvalidSpec
	case resource.ReasonReferenceNotFound:
		reason = ReasonReferenceNotFound
	case resource.ReasonPluginNotInstalled:
		reason = ReasonPluginNotInstalled
	case resource.ReasonInvalidReference:
		reason = ReasonInvalidReference
	case resource.ReasonConflict:
		reason = ReasonConflict
	case resource.ReasonUnsupported:
		reason = ReasonUnsupported
	case resource.ReasonCompileFailed:
		reason = ReasonCompileFailed
	case resource.ReasonArtifactUnavailable:
		reason = ReasonArtifactUnavailable
	case resource.ReasonRejected:
		reason = ReasonRejected
	case resource.ReasonDeliveryFailed:
		reason = ReasonDeliveryFailed
	}
	return Status{State: StateError, Reason: reason}
}

func disabledStatus() Status {
	return Status{State: StateDisabled, Reason: ReasonDisabled}
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
