package accesscontrol

import (
	"strconv"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func newAccessControlPolicyReply(policy *resource.AccessControlPolicy, names biz.PolicyTargetNames) *adminv1.AccessControlPolicy {
	status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := status.State == biz.ResourceStateDisabled
	reply := &adminv1.AccessControlPolicy{
		Id: policy.Name, Version: strconv.FormatInt(policy.Generation, 10), Status: adminservice.NewResourceStatus(status),
		Name: policy.Spec.DisplayName, Description: policy.Spec.Description, Enabled: policy.Spec.Enabled,
		Targets:       adminservice.NewPolicyTargets(policy.Generation, disabled, policy.Spec.TargetRefs, policy.Status.Targets, names),
		DefaultAction: string(policy.Spec.DefaultAction), CreatedAt: adminservice.NewTimestamp(policy.CreationTimestamp.Time),
		Response: &adminv1.AccessControlDenyResponse{
			StatusCode: int32(policy.Spec.Response.StatusCode), Message: policy.Spec.Response.Message,
		},
	}
	if reply.DefaultAction == "" {
		reply.DefaultAction = string(resource.AccessControlActionAllow)
	}
	if reply.Response.StatusCode == 0 {
		reply.Response.StatusCode = 403
	}
	if reply.Response.Message == "" {
		reply.Response.Message = "Access denied"
	}
	for _, item := range policy.Spec.Rules {
		rule := &adminv1.AccessControlRule{Name: item.Name, Action: string(item.Action)}
		for _, condition := range item.Conditions {
			rule.Conditions = append(rule.Conditions, &adminv1.AccessControlCondition{
				Type: string(condition.Type), Name: condition.Name, Value: condition.Value,
			})
		}
		reply.Rules = append(reply.Rules, rule)
	}
	return reply
}
