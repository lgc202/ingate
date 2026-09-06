package conversion

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/query"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
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
func ResourceState(state status.State) adminv1.ResourceState {
	switch state {
	case status.StateDisabled:
		return adminv1.ResourceState_DISABLED
	case status.StatePending:
		return adminv1.ResourceState_PENDING
	case status.StateReady:
		return adminv1.ResourceState_READY
	case status.StateError:
		return adminv1.ResourceState_ERROR
	default:
		return adminv1.ResourceState_RESOURCE_STATE_UNSPECIFIED
	}
}

// ResourceFilter 把控制台筛选条件转换为业务层查询条件。
func ResourceFilter(search string, enabled *bool, state adminv1.ResourceState) query.Filter {
	var resourceState status.State
	switch state {
	case adminv1.ResourceState_DISABLED:
		resourceState = status.StateDisabled
	case adminv1.ResourceState_PENDING:
		resourceState = status.StatePending
	case adminv1.ResourceState_READY:
		resourceState = status.StateReady
	case adminv1.ResourceState_ERROR:
		resourceState = status.StateError
	}
	return query.NewFilter(search, enabled, resourceState)
}

// ResourceMessage 返回控制台可以直接展示的资源状态文案。
func ResourceMessage(reason status.Reason) string {
	switch reason {
	case status.ReasonAwaitingAcceptance:
		return "配置正在处理中"
	case status.ReasonCheckingReferences:
		return "正在检查关联资源"
	case status.ReasonProgramming:
		return "配置正在生效"
	case status.ReasonReady:
		return "配置已生效"
	case status.ReasonDisabled:
		return "已停用"
	case status.ReasonUnapplied:
		return "配置已保存，尚未应用"
	case status.ReasonTargetNotApplied:
		return "目标当前没有可生效的流量入口"
	case status.ReasonInvalidSpec:
		return "配置内容不正确"
	case status.ReasonReferenceNotFound:
		return "引用的资源不存在"
	case status.ReasonPluginNotInstalled:
		return "依赖的插件未安装"
	case status.ReasonInvalidReference:
		return "引用的资源不可用"
	case status.ReasonConflict:
		return "配置与其他资源冲突"
	case status.ReasonUnsupported:
		return "当前版本尚不支持该配置"
	case status.ReasonCompileFailed:
		return "配置处理失败"
	case status.ReasonArtifactUnavailable:
		return "插件制品不可用"
	case status.ReasonRejected:
		return "配置未能生效"
	case status.ReasonDeliveryFailed:
		return "配置发布失败"
	default:
		return ""
	}
}
