package biz

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// PolicyTargetNames 保存策略作用目标的展示名称
type PolicyTargetNames map[PolicyTargetKey]string

// Name 返回目标引用对应的展示名称
func (n PolicyTargetNames) Name(ref resource.PolicyTargetRef) string {
	return n[PolicyTargetKey{Kind: ref.Kind, ID: ref.Name}]
}

// Contains 判断目标引用当前是否存在
func (n PolicyTargetNames) Contains(ref resource.PolicyTargetRef) bool {
	_, exists := n[PolicyTargetKey{Kind: ref.Kind, ID: ref.Name}]
	return exists
}

type RateLimitPolicyList struct {
	Policies    []resource.RateLimitPolicy
	TargetNames PolicyTargetNames
}

type RateLimitPolicyResult struct {
	Policy      *resource.RateLimitPolicy
	TargetNames PolicyTargetNames
}

type AccessControlPolicyList struct {
	Policies    []resource.AccessControlPolicy
	TargetNames PolicyTargetNames
}

type AccessControlPolicyResult struct {
	Policy      *resource.AccessControlPolicy
	TargetNames PolicyTargetNames
}

type TokenQuotaPolicyList struct {
	Policies    []resource.TokenQuotaPolicy
	TargetNames PolicyTargetNames
}

type TokenQuotaPolicyResult struct {
	Policy      *resource.TokenQuotaPolicy
	TargetNames PolicyTargetNames
}

type ConfigurationSummary struct {
	Total    int
	Ready    int
	Pending  int
	Error    int
	Disabled int
}

type ConfigurationItem struct {
	Kind   resource.Kind
	ID     string
	Name   string
	Status ResourceStatus
}

type ConfigurationReport struct {
	Summary ConfigurationSummary
	Items   []ConfigurationItem
}
