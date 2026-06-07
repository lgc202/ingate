package ratelimitpolicy

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 表示 RateLimitPolicy 列表查询结果
type ListResult struct {
	Policies []resource.RateLimitPolicy
}

// PolicyResult 表示单个 RateLimitPolicy 查询结果
type PolicyResult struct {
	Policy *resource.RateLimitPolicy
}

// PolicyParams 表示 RateLimitPolicy 可编辑字段
type PolicyParams struct {
	Name          string
	Description   string
	Enabled       bool
	Mode          resource.RateLimitMode
	Rules         []resource.RateLimitRule
	Global        *resource.GlobalRateLimitConfig
	Response      resource.RateLimitResponse
	FailurePolicy resource.RateLimitFailurePolicy
}

// CreatePolicyParams 表示创建 RateLimitPolicy 参数
type CreatePolicyParams struct {
	PolicyParams
}

// UpdatePolicyParams 表示更新 RateLimitPolicy 参数
type UpdatePolicyParams struct {
	Version string
	PolicyParams
}
