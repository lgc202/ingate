package accesscontrolpolicy

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// PolicyParams 表示 AccessControlPolicy 可编辑配置
type PolicyParams struct {
	Name          string
	Description   string
	Enabled       bool
	DefaultAction resource.AccessControlAction
	Rules         []resource.AccessControlRule
	Response      resource.AccessControlDenyResponse
}

// CreatePolicyParams 表示创建 AccessControlPolicy 参数
type CreatePolicyParams struct {
	PolicyParams
}

// UpdatePolicyParams 表示更新 AccessControlPolicy 参数
type UpdatePolicyParams struct {
	Version string
	PolicyParams
}

// ListResult 表示 AccessControlPolicy 列表结果
type ListResult struct {
	Policies []resource.AccessControlPolicy
}

// PolicyResult 表示单个 AccessControlPolicy 查询结果
type PolicyResult struct {
	Policy *resource.AccessControlPolicy
}
