package tokenquotapolicy

import (
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	"github.com/lgc202/ingate/internal/adminapi/service/resourcestatus"
	tokenquotapolicyservice "github.com/lgc202/ingate/internal/adminapi/service/tokenquotapolicy"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultRejectedMessage = "Token quota exceeded"
)

// NewListTokenQuotaPoliciesResp 转换 Token 配额策略列表用例结果为控制台响应
func NewListTokenQuotaPoliciesResp(result *tokenquotapolicyservice.ListResult) ListTokenQuotaPoliciesResp {
	policies := make([]TokenQuotaPolicy, 0, len(result.Policies))
	for i := range result.Policies {
		policies = append(policies, tokenQuotaPolicyFromResource(&result.Policies[i], result.TargetNames))
	}
	return ListTokenQuotaPoliciesResp{Policies: policies}
}

// NewGetTokenQuotaPolicyResp 转换单个 Token 配额策略用例结果为控制台响应
func NewGetTokenQuotaPolicyResp(result *tokenquotapolicyservice.PolicyResult) TokenQuotaPolicy {
	return tokenQuotaPolicyFromResource(result.Policy, result.TargetNames)
}

func tokenQuotaPolicyFromResource(
	policy *resource.TokenQuotaPolicy,
	targetNames policytarget.DisplayNames,
) TokenQuotaPolicy {
	status := resourcestatus.ForPolicy(policy.Generation, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := !policy.Spec.Enabled && resourcestatus.ConfigurationApplied(policy.Generation, policy.Status.Conditions)
	if disabled {
		status = resourcestatus.Disabled()
	}
	return TokenQuotaPolicy{
		ID:          policy.Name,
		Version:     strconv.FormatInt(policy.Generation, 10),
		Status:      admindto.NewResourceStatus(status),
		Name:        policy.Spec.DisplayName,
		Description: policy.Spec.Description,
		Enabled:     policy.Spec.Enabled,
		Targets:     tokenQuotaPolicyTargets(policy, targetNames, disabled),
		Subject: Subject{
			Type:       SubjectType(policy.Spec.Subject.Type),
			HeaderName: policy.Spec.Subject.HeaderName,
		},
		Quota: Quota{
			Tokens:        policy.Spec.Quota.Tokens,
			WindowSeconds: policy.Spec.Quota.WindowSeconds,
		},
		FailurePolicy: FailurePolicy(policy.Spec.FailurePolicy),
		Response:      tokenQuotaResponseFromResource(policy.Spec.Response),
		CreatedAt:     tokenQuotaCreatedAt(policy.ObjectMeta),
	}
}

func tokenQuotaPolicyTargets(
	policy *resource.TokenQuotaPolicy,
	targetNames policytarget.DisplayNames,
	disabled bool,
) []admindto.PolicyTarget {
	targets := make([]admindto.PolicyTarget, 0, len(policy.Spec.TargetRefs))
	for _, ref := range policy.Spec.TargetRefs {
		status := resourcestatus.ForPolicyTarget(policy.Generation, tokenQuotaTargetConditions(policy.Status.Targets, ref))
		if disabled {
			status = resourcestatus.Disabled()
		}
		targets = append(targets, admindto.PolicyTarget{
			Kind:        admindto.PolicyTargetKind(ref.Kind),
			ID:          ref.Name,
			DisplayName: targetNames.Name(ref),
			Status:      admindto.NewResourceStatus(status),
		})
	}
	return targets
}

func tokenQuotaTargetConditions(statuses []resource.PolicyTargetStatus, ref resource.PolicyTargetRef) []metav1.Condition {
	for _, status := range statuses {
		if status.TargetRef.Kind == ref.Kind && status.TargetRef.Name == ref.Name {
			return status.Conditions
		}
	}
	return nil
}

func tokenQuotaResponseFromResource(response resource.TokenQuotaResponse) Response {
	result := Response{Message: response.Message}
	if result.Message == "" {
		result.Message = defaultRejectedMessage
	}
	return result
}

func tokenQuotaCreatedAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
