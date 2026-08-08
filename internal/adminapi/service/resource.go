package service

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
)

func timestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func optionalTime(value *timestamppb.Timestamp) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if err := value.CheckValid(); err != nil {
		return nil, badRequest("时间格式不正确")
	}
	result := value.AsTime()
	return &result, nil
}

func resourceStatus(status biz.ResourceStatus) *adminv1.ResourceStatus {
	message := ""
	switch status.Reason {
	case biz.ReasonAwaitingAcceptance:
		message = "配置正在处理中"
	case biz.ReasonCheckingReferences:
		message = "正在检查关联资源"
	case biz.ReasonProgramming:
		message = "配置正在生效"
	case biz.ReasonReady:
		message = "配置已生效"
	case biz.ReasonDisabled:
		message = "已停用"
	case biz.ReasonUnapplied:
		message = "策略已保存，尚未应用"
	case biz.ReasonTargetNotApplied:
		message = "目标当前没有可生效的流量入口"
	case biz.ReasonInvalidSpec:
		message = "配置内容不正确"
	case biz.ReasonReferenceNotFound:
		message = "引用的资源不存在"
	case biz.ReasonInvalidReference:
		message = "引用的资源不可用"
	case biz.ReasonConflict:
		message = "配置与其他资源冲突"
	case biz.ReasonUnsupported:
		message = "当前版本尚不支持该配置"
	case biz.ReasonCompileFailed:
		message = "配置处理失败"
	case biz.ReasonRejected:
		message = "配置未能生效"
	case biz.ReasonDeliveryFailed:
		message = "配置发布失败"
	}
	return &adminv1.ResourceStatus{State: string(status.State), Message: message}
}
