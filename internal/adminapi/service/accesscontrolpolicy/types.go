package accesscontrolpolicy

import (
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// PolicyParams 表示 AccessControlPolicy 可编辑配置
type PolicyParams struct {
	Name          string
	Description   string
	Enabled       bool
	Targets       []TargetParams
	DefaultAction resource.AccessControlAction
	Rules         []resource.AccessControlRule
	Response      resource.AccessControlDenyResponse
}

// TargetParams 表示策略作用目标参数
type TargetParams struct {
	Kind resource.Kind
	ID   string
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
	Policies    []resource.AccessControlPolicy
	TargetNames policytarget.DisplayNames
}

// PolicyResult 表示单个 AccessControlPolicy 查询结果
type PolicyResult struct {
	Policy      *resource.AccessControlPolicy
	TargetNames policytarget.DisplayNames
}
