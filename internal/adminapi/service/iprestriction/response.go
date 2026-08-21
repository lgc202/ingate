package iprestriction

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func ipRestrictionPolicyResponse(
	policy *resource.IPRestrictionPolicy,
	names biz.PolicyTargetNames,
) *adminv1.IPRestrictionPolicy {
	status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := status.State == biz.ResourceStateDisabled
	return &adminv1.IPRestrictionPolicy{
		Id:      policy.Name,
		Name:    policy.Spec.DisplayName,
		Enabled: policy.Spec.Enabled,
		Targets: adminservice.PolicyTargetResponses(
			policy.Generation,
			disabled,
			policy.Spec.TargetRefs,
			policy.Status.Targets,
			names,
		),
		Allow:     append([]string(nil), policy.Spec.Allow...),
		Deny:      append([]string(nil), policy.Spec.Deny...),
		State:     adminservice.ResourceState(status.State),
		Message:   adminservice.ResourceMessage(status.Reason),
		Version:   policy.Generation,
		CreatedAt: adminservice.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt: adminservice.Timestamp(adminservice.ResourceUpdatedAt(policy.Annotations)),
	}
}
