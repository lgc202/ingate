package dto

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// AccessControlPolicyConfig 是控制台读写 AccessControlPolicy 时复用的核心配置
type AccessControlPolicyConfig struct {
	Name          string                             `json:"name"`
	Description   string                             `json:"description,omitempty"`
	Enabled       bool                               `json:"enabled"`
	DefaultAction resource.AccessControlAction       `json:"defaultAction,omitempty"`
	Rules         []resource.AccessControlRule       `json:"rules,omitempty"`
	Response      resource.AccessControlDenyResponse `json:"response,omitempty"`
}

// CreateAccessControlPolicyReq 是创建访问控制策略的请求体
type CreateAccessControlPolicyReq struct {
	AccessControlPolicyConfig
}

// UpdateAccessControlPolicyReq 是更新访问控制策略的请求体
type UpdateAccessControlPolicyReq struct {
	Version string `json:"version"`
	AccessControlPolicyConfig
}

// SetEnabledReq 是启停访问控制策略的请求体
type SetEnabledReq struct {
	Enabled bool `json:"enabled"`
}

// AccessControlPolicy 是 admin-api 面向控制台返回的访问控制策略
type AccessControlPolicy struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
	AccessControlPolicyConfig
}

// ListAccessControlPoliciesResp 是访问控制策略列表响应
type ListAccessControlPoliciesResp struct {
	Policies []AccessControlPolicy `json:"policies"`
}

// CreateAccessControlPolicyResp 是创建访问控制策略响应
type CreateAccessControlPolicyResp struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
}

// UpdateAccessControlPolicyResp 是更新访问控制策略响应
type UpdateAccessControlPolicyResp struct {
	Success bool `json:"success"`
}

// DeleteAccessControlPolicyResp 是删除访问控制策略响应
type DeleteAccessControlPolicyResp struct {
	Success bool `json:"success"`
}
