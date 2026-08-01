// Package accesscontrolpolicy 定义访问控制策略管理接口的请求和响应模型
package accesscontrolpolicy

import (
	admindto "github.com/lgc202/ingate/internal/admin/dto"
)

// Action 表示访问控制规则命中后的处理动作
type Action string

const (
	ActionAllow Action = "Allow"
	ActionDeny  Action = "Deny"
)

// ConditionType 表示访问控制匹配维度
type ConditionType string

const (
	ConditionTypeIP     ConditionType = "IP"
	ConditionTypeHeader ConditionType = "Header"
)

// AccessControlPolicyConfig 是控制台创建和更新访问控制策略的产品配置
type AccessControlPolicyConfig struct {
	Name          string                     `json:"name"`
	Description   string                     `json:"description,omitempty"`
	Enabled       bool                       `json:"enabled"`
	Targets       []admindto.PolicyTargetReq `json:"targets,omitempty"`
	DefaultAction Action                     `json:"defaultAction,omitempty"`
	Rules         []Rule                     `json:"rules,omitempty"`
	Response      DenyResponse               `json:"response,omitempty"`
}

// Rule 表示一条访问控制规则
type Rule struct {
	Name       string      `json:"name"`
	Action     Action      `json:"action"`
	Conditions []Condition `json:"conditions,omitempty"`
}

// Condition 表示访问控制规则中的一个匹配条件
type Condition struct {
	Type  ConditionType `json:"type"`
	Name  string        `json:"name,omitempty"`
	Value string        `json:"value"`
}

// DenyResponse 表示访问被拒绝时返回给调用方的响应
type DenyResponse struct {
	StatusCode int    `json:"statusCode,omitempty"`
	Message    string `json:"message,omitempty"`
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
	Enabled *bool `json:"enabled"`
}

// AccessControlPolicy 是 Admin API 返回的访问控制策略
type AccessControlPolicy struct {
	ID            string                  `json:"id"`
	Version       string                  `json:"version"`
	Status        admindto.ResourceStatus `json:"status"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description,omitempty"`
	Enabled       bool                    `json:"enabled"`
	Targets       []admindto.PolicyTarget `json:"targets"`
	DefaultAction Action                  `json:"defaultAction,omitempty"`
	Rules         []Rule                  `json:"rules,omitempty"`
	Response      DenyResponse            `json:"response,omitempty"`
	CreatedAt     string                  `json:"createdAt"`
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
