package accesscontrolpolicy

import (
	"time"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
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
	status := admindto.NewResourceStatus(policy.Generation, policy.Status.Conditions)
	if !policy.Spec.Enabled {
		status = admindto.NewDisabledResourceStatus()
	}
	return AccessControlPolicy{
		ID:      policy.Name,
		Version: policy.ResourceVersion,
		Status:  status,
		AccessControlPolicyConfig: AccessControlPolicyConfig{
			Name:          policy.Spec.DisplayName,
			Description:   policy.Spec.Description,
			Enabled:       policy.Spec.Enabled,
			DefaultAction: policy.Spec.DefaultAction,
			Rules:         policy.Spec.Rules,
			Response:      policy.Spec.Response,
		},
		CreatedAt: createdAt(policy.ObjectMeta),
	}
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
