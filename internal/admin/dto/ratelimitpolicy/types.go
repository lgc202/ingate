// Package ratelimitpolicy 定义限流策略管理接口的请求和响应模型
package ratelimitpolicy

import (
	admindto "github.com/lgc202/ingate/internal/admin/dto"
)

// KeyType 表示控制台可配置的限流计数维度
type KeyType string

const (
	KeyTypeIP        KeyType = "IP"
	KeyTypeHeader    KeyType = "Header"
	KeyTypeQuery     KeyType = "Query"
	KeyTypeCookie    KeyType = "Cookie"
	KeyTypeRoute     KeyType = "Route"
	KeyTypeGateway   KeyType = "Gateway"
	KeyTypeRouteRule KeyType = "RouteRule"
)

// FailurePolicy 表示限流执行异常时的请求处理方式
type FailurePolicy string

const (
	FailurePolicyFailOpen  FailurePolicy = "FailOpen"
	FailurePolicyFailClose FailurePolicy = "FailClose"
)

// RateLimitPolicyConfig 是控制台创建和更新限流策略的产品配置
type RateLimitPolicyConfig struct {
	Name          string                     `json:"name"`
	Description   string                     `json:"description,omitempty"`
	Enabled       bool                       `json:"enabled"`
	Targets       []admindto.PolicyTargetReq `json:"targets,omitempty"`
	Rules         []Rule                     `json:"rules"`
	Response      Response                   `json:"response,omitempty"`
	FailurePolicy FailurePolicy              `json:"failurePolicy,omitempty"`
}

// Rule 表示一条限流额度规则
type Rule struct {
	Name  string `json:"name"`
	Key   Key    `json:"key"`
	Limit Quota  `json:"limit"`
}

// Key 表示限流计数 key 的组成方式
type Key struct {
	Parts []KeyPart `json:"parts"`
}

// KeyPart 表示限流计数 key 的一个组成部分
type KeyPart struct {
	Type KeyType `json:"type"`
	Name string  `json:"name,omitempty"`
}

// Quota 表示窗口内允许的请求额度，Burst 为 0 时使用默认容量
type Quota struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
	Burst         int `json:"burst,omitempty"`
}

// Response 表示超过限流额度时返回给调用方的响应
type Response struct {
	StatusCode         int    `json:"statusCode,omitempty"`
	Message            string `json:"message,omitempty"`
	QuotaHeaderEnabled bool   `json:"quotaHeaderEnabled"`
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
	Enabled *bool `json:"enabled"`
}

// RateLimitPolicy 是 Admin API 返回的限流策略
type RateLimitPolicy struct {
	ID            string                  `json:"id"`
	Version       string                  `json:"version"`
	Status        admindto.ResourceStatus `json:"status"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description,omitempty"`
	Enabled       bool                    `json:"enabled"`
	Targets       []admindto.PolicyTarget `json:"targets"`
	Rules         []Rule                  `json:"rules"`
	Response      Response                `json:"response,omitempty"`
	FailurePolicy FailurePolicy           `json:"failurePolicy,omitempty"`
	CreatedAt     string                  `json:"createdAt"`
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
