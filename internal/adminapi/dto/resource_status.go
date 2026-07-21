// Package dto 定义多个 Admin API 资源共享的控制台请求和响应模型
package dto

import "github.com/lgc202/ingate/internal/adminapi/service/resourcestatus"

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

const (
	messageProcessing         = "配置正在处理中"
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

// ResourceStatus 是控制台使用的声明式资源状态
type ResourceStatus struct {
	State   ResourceState `json:"state"`
	Message string        `json:"message"`
}

// NewResourceStatus 将 service 状态转换为控制台响应结构
func NewResourceStatus(status resourcestatus.Status) ResourceStatus {
	var result ResourceStatus
	switch status.State {
	case resourcestatus.StatePending:
		result.State = ResourceStatePending
	case resourcestatus.StateReady:
		result.State = ResourceStateReady
	case resourcestatus.StateError:
		result.State = ResourceStateError
	case resourcestatus.StateDisabled:
		result.State = ResourceStateDisabled
	}

	switch status.Reason {
	case resourcestatus.ReasonAwaitingAcceptance:
		result.Message = messageProcessing
	case resourcestatus.ReasonCheckingReferences:
		result.Message = messageCheckingReferences
	case resourcestatus.ReasonProgramming:
		result.Message = messageProgramming
	case resourcestatus.ReasonReady:
		result.Message = messageReady
	case resourcestatus.ReasonDisabled:
		result.Message = messageDisabled
	case resourcestatus.ReasonUnapplied:
		result.Message = messageUnapplied
	case resourcestatus.ReasonTargetNotApplied:
		result.Message = messageTargetNotApplied
	case resourcestatus.ReasonInvalidSpec:
		result.Message = messageInvalidSpec
	case resourcestatus.ReasonReferenceNotFound:
		result.Message = messageReferenceNotFound
	case resourcestatus.ReasonInvalidReference:
		result.Message = messageInvalidReference
	case resourcestatus.ReasonConflict:
		result.Message = messageConflict
	case resourcestatus.ReasonUnsupported:
		result.Message = messageUnsupported
	case resourcestatus.ReasonCompileFailed:
		result.Message = messageCompileFailed
	case resourcestatus.ReasonRejected:
		result.Message = messageRejected
	case resourcestatus.ReasonDeliveryFailed:
		result.Message = messageDeliveryFailed
	}
	return result
}
