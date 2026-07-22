// Package tokenquotapolicy 定义 Token 配额策略管理接口的请求和响应模型
package tokenquotapolicy

import admindto "github.com/lgc202/ingate/internal/adminapi/dto"

// SubjectType 表示 Token 预算池的共享维度
type SubjectType string

const (
	// SubjectTypeShared 表示所有命中请求共享额度
	SubjectTypeShared SubjectType = "Shared"
	// SubjectTypeIP 表示每个来源 IP 独立累计额度
	SubjectTypeIP SubjectType = "IP"
	// SubjectTypeHeader 表示每个请求 Header 值独立累计额度
	SubjectTypeHeader SubjectType = "Header"
)

// FailurePolicy 表示配额状态不可用时的请求处理方式
type FailurePolicy string

const (
	// FailurePolicyFailOpen 表示配额状态不可用时放行请求
	FailurePolicyFailOpen FailurePolicy = "FailOpen"
	// FailurePolicyFailClose 表示配额状态不可用时拒绝请求
	FailurePolicyFailClose FailurePolicy = "FailClose"
)

// TokenQuotaPolicyConfig 是控制台创建和更新 Token 配额策略的产品配置
type TokenQuotaPolicyConfig struct {
	Name          string                     `json:"name"`
	Description   string                     `json:"description,omitempty"`
	Enabled       bool                       `json:"enabled"`
	Targets       []admindto.PolicyTargetReq `json:"targets,omitempty"`
	Subject       Subject                    `json:"subject"`
	Quota         Quota                      `json:"quota"`
	FailurePolicy FailurePolicy              `json:"failurePolicy"`
	Response      Response                   `json:"response,omitempty"`
}

// IDReq 是 TokenQuotaPolicy 路径参数
type IDReq struct {
	ID string `uri:"id"`
}

// Subject 定义请求如何映射到 Token 预算池
type Subject struct {
	Type       SubjectType `json:"type"`
	HeaderName string      `json:"headerName,omitempty"`
}

// Quota 表示一个统计窗口内允许消耗的 Token 总数
type Quota struct {
	Tokens        int64 `json:"tokens"`
	WindowSeconds int64 `json:"windowSeconds"`
}

// Response 表示超过 Token 配额时返回给调用方的响应
type Response struct {
	Message string `json:"message,omitempty"`
}

// CreateTokenQuotaPolicyReq 是创建 TokenQuotaPolicy 的请求体
type CreateTokenQuotaPolicyReq struct {
	TokenQuotaPolicyConfig
}

// UpdateTokenQuotaPolicyReq 是更新 TokenQuotaPolicy 的请求体
type UpdateTokenQuotaPolicyReq struct {
	Version string `json:"version"`
	TokenQuotaPolicyConfig
}

// SetEnabledReq 是设置启用状态的请求体
type SetEnabledReq struct {
	Enabled *bool `json:"enabled"`
}

// TokenQuotaPolicy 是 admin-api 面向控制台返回的 Token 配额策略
type TokenQuotaPolicy struct {
	ID            string                  `json:"id"`
	Version       string                  `json:"version"`
	Status        admindto.ResourceStatus `json:"status"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description,omitempty"`
	Enabled       bool                    `json:"enabled"`
	Targets       []admindto.PolicyTarget `json:"targets"`
	Subject       Subject                 `json:"subject"`
	Quota         Quota                   `json:"quota"`
	FailurePolicy FailurePolicy           `json:"failurePolicy"`
	Response      Response                `json:"response"`
	CreatedAt     string                  `json:"createdAt"`
}

// ListTokenQuotaPoliciesResp 是 Token 配额策略列表响应
type ListTokenQuotaPoliciesResp struct {
	Policies []TokenQuotaPolicy `json:"policies"`
}

// CreateTokenQuotaPolicyResp 是创建 Token 配额策略响应
type CreateTokenQuotaPolicyResp struct {
	Success bool   `json:"success"`
	ID      string `json:"id,omitempty"`
}

// UpdateTokenQuotaPolicyResp 是更新 Token 配额策略响应
type UpdateTokenQuotaPolicyResp struct {
	Success bool `json:"success"`
}

// SetEnabledResp 是设置启用状态响应
type SetEnabledResp struct {
	Success bool `json:"success"`
}

// DeleteTokenQuotaPolicyResp 是删除 Token 配额策略响应
type DeleteTokenQuotaPolicyResp struct {
	Success bool `json:"success"`
}
