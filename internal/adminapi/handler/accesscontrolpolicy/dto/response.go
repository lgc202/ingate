package dto

import (
	"github.com/samber/lo"

	accesscontrolpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/accesscontrolpolicy"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewListAccessControlPoliciesResp 转换访问控制策略列表用例结果为控制台响应
func NewListAccessControlPoliciesResp(result *accesscontrolpolicyservice.ListResult) ListAccessControlPoliciesResp {
	return ListAccessControlPoliciesResp{
		Policies: lo.Map(result.Policies, func(policy resource.AccessControlPolicy, _ int) AccessControlPolicy {
			return policyFromResource(&policy)
		}),
	}
}

// NewGetAccessControlPolicyResp 转换单个访问控制策略用例结果为控制台响应
func NewGetAccessControlPolicyResp(result *accesscontrolpolicyservice.PolicyResult) AccessControlPolicy {
	return policyFromResource(result.Policy)
}

func policyFromResource(policy *resource.AccessControlPolicy) AccessControlPolicy {
	return AccessControlPolicy{
		ID:      policy.Name,
		Version: policy.ResourceVersion,
		AccessControlPolicyConfig: AccessControlPolicyConfig{
			Name:          policy.Spec.DisplayName,
			Description:   policy.Spec.Description,
			Enabled:       policy.Spec.Enabled,
			DefaultAction: policy.Spec.DefaultAction,
			Rules:         policy.Spec.Rules,
			Response:      policy.Spec.Response,
		},
	}
}
