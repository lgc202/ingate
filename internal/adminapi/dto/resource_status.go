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

// ResourceStatus 是控制台使用的声明式资源状态
type ResourceStatus struct {
	State   ResourceState `json:"state"`
	Message string        `json:"message"`
}

// NewResourceStatus 将 service 状态转换为控制台响应结构
func NewResourceStatus(status resourcestatus.Status) ResourceStatus {
	return ResourceStatus{
		State:   ResourceState(status.State),
		Message: status.Message,
	}
}
