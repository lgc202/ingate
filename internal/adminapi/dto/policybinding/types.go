// Package policybinding 定义策略绑定管理接口的请求和响应模型
package policybinding

import (
	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// PolicyBindingConfig 是控制台读写 PolicyBinding 时复用的核心配置
type PolicyBindingConfig struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	Enabled     bool                     `json:"enabled"`
	TargetRef   resource.PolicyTargetRef `json:"targetRef"`
	Policies    []resource.PolicyRef     `json:"policies"`
}

// CreatePolicyBindingReq 是创建 PolicyBinding 的请求体
type CreatePolicyBindingReq struct {
	PolicyBindingConfig
}

// UpdatePolicyBindingReq 是更新 PolicyBinding 的请求体
type UpdatePolicyBindingReq struct {
	Version string `json:"version"`
	PolicyBindingConfig
}

// SetEnabledReq 是设置启用状态的请求体
type SetEnabledReq struct {
	Enabled bool `json:"enabled"`
}

// PolicyBinding 是 admin-api 面向控制台返回的策略绑定
type PolicyBinding struct {
	ID      string                  `json:"id"`
	Version string                  `json:"version,omitempty"`
	Status  admindto.ResourceStatus `json:"status"`
	PolicyBindingConfig
	CreatedAt string `json:"createdAt"`
}

// ListPolicyBindingsResp 是策略绑定列表响应
type ListPolicyBindingsResp struct {
	Bindings []PolicyBinding `json:"bindings"`
}

// CreatePolicyBindingResp 是创建策略绑定响应
type CreatePolicyBindingResp struct {
	Success bool   `json:"success"`
	ID      string `json:"id,omitempty"`
}

// UpdatePolicyBindingResp 是更新策略绑定响应
type UpdatePolicyBindingResp struct {
	Success bool `json:"success"`
}

// SetEnabledResp 是设置启用状态响应
type SetEnabledResp struct {
	Success bool `json:"success"`
}

// DeletePolicyBindingResp 是删除策略绑定响应
type DeletePolicyBindingResp struct {
	Success bool `json:"success"`
}
