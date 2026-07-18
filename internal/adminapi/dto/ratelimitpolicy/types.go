// Package ratelimitpolicy 定义限流策略管理接口的请求和响应模型
package ratelimitpolicy

import (
	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// RateLimitPolicyConfig 是控制台读写 RateLimitPolicy 时复用的核心配置
type RateLimitPolicyConfig struct {
	Name          string                          `json:"name"`
	Description   string                          `json:"description,omitempty"`
	Enabled       bool                            `json:"enabled"`
	Mode          resource.RateLimitMode          `json:"mode"`
	Rules         []resource.RateLimitRule        `json:"rules"`
	Response      resource.RateLimitResponse      `json:"response,omitempty"`
	FailurePolicy resource.RateLimitFailurePolicy `json:"failurePolicy,omitempty"`
}

// CreateRateLimitPolicyReq 是创建 RateLimitPolicy 的请求体
type CreateRateLimitPolicyReq struct {
	RateLimitPolicyConfig
}

// UpdateRateLimitPolicyReq 是更新 RateLimitPolicy 的请求体
type UpdateRateLimitPolicyReq struct {
	Version string `json:"version"`
	RateLimitPolicyConfig
}

// SetEnabledReq 是设置启用状态的请求体
type SetEnabledReq struct {
	Enabled bool `json:"enabled"`
}

// RateLimitPolicy 是 admin-api 面向控制台返回的限流策略
type RateLimitPolicy struct {
	ID      string                  `json:"id"`
	Version string                  `json:"version,omitempty"`
	Status  admindto.ResourceStatus `json:"status"`
	RateLimitPolicyConfig
	CreatedAt string `json:"createdAt"`
}

// ListRateLimitPoliciesResp 是限流策略列表响应
type ListRateLimitPoliciesResp struct {
	Policies []RateLimitPolicy `json:"policies"`
}

// CreateRateLimitPolicyResp 是创建限流策略响应
type CreateRateLimitPolicyResp struct {
	Success bool   `json:"success"`
	ID      string `json:"id,omitempty"`
}

// UpdateRateLimitPolicyResp 是更新限流策略响应
type UpdateRateLimitPolicyResp struct {
	Success bool `json:"success"`
}

// SetEnabledResp 是设置启用状态响应
type SetEnabledResp struct {
	Success bool `json:"success"`
}

// DeleteRateLimitPolicyResp 是删除限流策略响应
type DeleteRateLimitPolicyResp struct {
	Success bool `json:"success"`
}
