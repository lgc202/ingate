package iprestriction

import (
	"time"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func ipRestrictionPolicyFromResource(
	policy *resource.IPRestrictionPolicy,
	names biz.PolicyTargetNames,
) *adminv1.IPRestrictionPolicy {
	status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := status.State == biz.ResourceStateDisabled
	return &adminv1.IPRestrictionPolicy{
		Id:      policy.Name,
		Name:    policy.Spec.DisplayName,
		Enabled: policy.Spec.Enabled,
		Targets: adminservice.NewPolicyTargets(
			policy.Generation,
			disabled,
			policy.Spec.TargetRefs,
			policy.Status.Targets,
			names,
		),
		Allow:     policy.Spec.Allow,
		Deny:      policy.Spec.Deny,
		State:     adminservice.NewResourceState(status.State),
		Message:   adminservice.ResourceMessage(status.Reason),
		Version:   policy.Generation,
		CreatedAt: adminservice.NewTimestamp(policy.CreationTimestamp.Time),
		UpdatedAt: adminservice.NewTimestamp(ipRestrictionPolicyUpdatedAt(policy)),
	}
}

func ipRestrictionPolicyUpdatedAt(policy *resource.IPRestrictionPolicy) time.Time {
	value := policy.Annotations[resource.AnnotationUpdatedAt]
	if value == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
