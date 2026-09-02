package protocol

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Timestamp 把非零时间转换为协议时间。
func Timestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

// ResourceUpdatedAt 读取由 API Server 注解维护的资源更新时间。
func ResourceUpdatedAt(annotations map[string]string) time.Time {
	value := annotations[resource.AnnotationUpdatedAt]
	if value == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

// ResourceState 把领域状态转换为控制台协议枚举。
func ResourceState(state resourceview.State) adminv1.ResourceState {
	switch state {
	case resourceview.StateDisabled:
		return adminv1.ResourceState_DISABLED
	case resourceview.StatePending:
		return adminv1.ResourceState_PENDING
	case resourceview.StateReady:
		return adminv1.ResourceState_READY
	case resourceview.StateError:
		return adminv1.ResourceState_ERROR
	default:
		return adminv1.ResourceState_RESOURCE_STATE_UNSPECIFIED
	}
}

// ResourceFilter 把控制台筛选条件转换为业务层查询条件。
func ResourceFilter(query string, enabled *bool, state adminv1.ResourceState) resourceview.Filter {
	var resourceState resourceview.State
	switch state {
	case adminv1.ResourceState_DISABLED:
		resourceState = resourceview.StateDisabled
	case adminv1.ResourceState_PENDING:
		resourceState = resourceview.StatePending
	case adminv1.ResourceState_READY:
		resourceState = resourceview.StateReady
	case adminv1.ResourceState_ERROR:
		resourceState = resourceview.StateError
	}
	return resourceview.NewFilter(query, enabled, resourceState)
}

// ResourceMessage 返回控制台可以直接展示的资源状态文案。
func ResourceMessage(reason resourceview.Reason) string {
	switch reason {
	case resourceview.ReasonAwaitingAcceptance:
		return "配置正在处理中"
	case resourceview.ReasonCheckingReferences:
		return "正在检查关联资源"
	case resourceview.ReasonProgramming:
		return "配置正在生效"
	case resourceview.ReasonReady:
		return "配置已生效"
	case resourceview.ReasonDisabled:
		return "已停用"
	case resourceview.ReasonUnapplied:
		return "配置已保存，尚未应用"
	case resourceview.ReasonTargetNotApplied:
		return "目标当前没有可生效的流量入口"
	case resourceview.ReasonInvalidSpec:
		return "配置内容不正确"
	case resourceview.ReasonReferenceNotFound:
		return "引用的资源不存在"
	case resourceview.ReasonPluginNotInstalled:
		return "依赖的插件未安装"
	case resourceview.ReasonInvalidReference:
		return "引用的资源不可用"
	case resourceview.ReasonConflict:
		return "配置与其他资源冲突"
	case resourceview.ReasonUnsupported:
		return "当前版本尚不支持该配置"
	case resourceview.ReasonCompileFailed:
		return "配置处理失败"
	case resourceview.ReasonArtifactUnavailable:
		return "插件制品不可用"
	case resourceview.ReasonRejected:
		return "配置未能生效"
	case resourceview.ReasonDeliveryFailed:
		return "配置发布失败"
	default:
		return ""
	}
}
