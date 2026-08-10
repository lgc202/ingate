package service

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
)

// NewTimestamp 把非零时间转换为协议时间
func NewTimestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

// TimeFromProto 校验并转换可选协议时间
func TimeFromProto(value *timestamppb.Timestamp) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if err := value.CheckValid(); err != nil {
		return nil, BadRequest("时间格式不正确")
	}
	result := value.AsTime()
	return &result, nil
}

// NewResourceStatus 把领域状态转换为控制台状态
func NewResourceStatus(status biz.ResourceStatus) *adminv1.ResourceStatus {
	return &adminv1.ResourceStatus{State: NewResourceState(status.State), Message: ResourceMessage(status.Reason)}
}

// NewResourceState 把领域状态转换为控制台协议枚举
func NewResourceState(state biz.ResourceState) adminv1.ResourceState {
	switch state {
	case biz.ResourceStateDisabled:
		return adminv1.ResourceState_DISABLED
	case biz.ResourceStatePending:
		return adminv1.ResourceState_PENDING
	case biz.ResourceStateReady:
		return adminv1.ResourceState_READY
	case biz.ResourceStateError:
		return adminv1.ResourceState_ERROR
	default:
		return adminv1.ResourceState_RESOURCE_STATE_UNSPECIFIED
	}
}

// ResourceMessage 返回控制台可以直接展示的资源状态文案
func ResourceMessage(reason biz.ResourceReason) string {
	switch reason {
	case biz.ReasonAwaitingAcceptance:
		return "配置正在处理中"
	case biz.ReasonCheckingReferences:
		return "正在检查关联资源"
	case biz.ReasonProgramming:
		return "配置正在生效"
	case biz.ReasonReady:
		return "配置已生效"
	case biz.ReasonDisabled:
		return "已停用"
	case biz.ReasonUnapplied:
		return "策略已保存，尚未应用"
	case biz.ReasonTargetNotApplied:
		return "目标当前没有可生效的流量入口"
	case biz.ReasonInvalidSpec:
		return "配置内容不正确"
	case biz.ReasonReferenceNotFound:
		return "引用的资源不存在"
	case biz.ReasonInvalidReference:
		return "引用的资源不可用"
	case biz.ReasonConflict:
		return "配置与其他资源冲突"
	case biz.ReasonUnsupported:
		return "当前版本尚不支持该配置"
	case biz.ReasonCompileFailed:
		return "配置处理失败"
	case biz.ReasonRejected:
		return "配置未能生效"
	case biz.ReasonDeliveryFailed:
		return "配置发布失败"
	default:
		return ""
	}
}
