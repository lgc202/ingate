package accesscontrolpolicy

import (
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	accesscontrolpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/accesscontrolpolicy"
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	"github.com/lgc202/ingate/internal/adminapi/service/resourcestatus"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultAction             = ActionAllow
	defaultDeniedStatusCode   = 403
	defaultDeniedResponseText = "Access denied"
)

// NewListAccessControlPoliciesResp 转换访问控制策略列表用例结果为控制台响应
func NewListAccessControlPoliciesResp(result *accesscontrolpolicyservice.ListResult) ListAccessControlPoliciesResp {
	policies := make([]AccessControlPolicy, 0, len(result.Policies))
	for i := range result.Policies {
		policies = append(policies, policyFromResource(&result.Policies[i], result.TargetNames))
	}
	return ListAccessControlPoliciesResp{Policies: policies}
}

// NewGetAccessControlPolicyResp 转换单个访问控制策略用例结果为控制台响应
func NewGetAccessControlPolicyResp(result *accesscontrolpolicyservice.PolicyResult) AccessControlPolicy {
	return policyFromResource(result.Policy, result.TargetNames)
}

func policyFromResource(policy *resource.AccessControlPolicy, targetNames policytarget.DisplayNames) AccessControlPolicy {
	status := resourcestatus.ForPolicy(policy.Generation, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := !policy.Spec.Enabled && resourcestatus.ConfigurationApplied(policy.Generation, policy.Status.Conditions)
	if disabled {
		status = resourcestatus.Disabled()
	}
	action := Action(policy.Spec.DefaultAction)
	if action == "" {
		action = defaultAction
	}
	response := DenyResponse{
		StatusCode: policy.Spec.Response.StatusCode,
		Message:    policy.Spec.Response.Message,
	}
	if response.StatusCode == 0 {
		response.StatusCode = defaultDeniedStatusCode
	}
	if response.Message == "" {
		response.Message = defaultDeniedResponseText
	}
	return AccessControlPolicy{
		ID:            policy.Name,
		Version:       strconv.FormatInt(policy.Generation, 10),
		Status:        admindto.NewResourceStatus(status),
		Name:          policy.Spec.DisplayName,
		Description:   policy.Spec.Description,
		Enabled:       policy.Spec.Enabled,
		Targets:       policyTargets(policy, targetNames, disabled),
		DefaultAction: action,
		Rules:         rulesFromResource(policy.Spec.Rules),
		Response:      response,
		CreatedAt:     createdAt(policy.ObjectMeta),
	}
}

func policyTargets(
	policy *resource.AccessControlPolicy,
	targetNames policytarget.DisplayNames,
	disabled bool,
) []admindto.PolicyTarget {
	targets := make([]admindto.PolicyTarget, 0, len(policy.Spec.TargetRefs))
	for _, ref := range policy.Spec.TargetRefs {
		status := resourcestatus.ForPolicyTarget(policy.Generation, targetConditions(policy.Status.Targets, ref))
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

func targetConditions(statuses []resource.PolicyTargetStatus, ref resource.PolicyTargetRef) []metav1.Condition {
	for _, status := range statuses {
		if status.TargetRef.Kind == ref.Kind && status.TargetRef.Name == ref.Name {
			return status.Conditions
		}
	}
	return nil
}

func rulesFromResource(items []resource.AccessControlRule) []Rule {
	rules := make([]Rule, 0, len(items))
	for _, item := range items {
		conditions := make([]Condition, 0, len(item.Conditions))
		for _, condition := range item.Conditions {
			conditions = append(conditions, Condition{
				Type:  ConditionType(condition.Type),
				Name:  condition.Name,
				Value: condition.Value,
			})
		}
		rules = append(rules, Rule{
			Name:       item.Name,
			Action:     Action(item.Action),
			Conditions: conditions,
		})
	}
	return rules
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
