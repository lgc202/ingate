package headertransformation

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func headerTransformationPolicyResponse(
	policy *resource.HeaderTransformationPolicy,
	names biz.PolicyTargetNames,
) *adminv1.HeaderTransformationPolicy {
	status := biz.PolicyStatus(
		policy.Generation,
		policy.Spec.Enabled,
		len(policy.Spec.TargetRefs),
		policy.Status.Conditions,
	)
	disabled := status.State == biz.ResourceStateDisabled
	return &adminv1.HeaderTransformationPolicy{
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
		RequestRules:  headerTransformationRuleResponses(policy.Spec.RequestRules),
		ResponseRules: headerTransformationRuleResponses(policy.Spec.ResponseRules),
		State:         adminservice.ResourceState(status.State),
		Message:       adminservice.ResourceMessage(status.Reason),
		Version:       policy.Generation,
		CreatedAt:     adminservice.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt:     adminservice.Timestamp(adminservice.ResourceUpdatedAt(policy.Annotations)),
	}
}

func headerTransformationRuleResponses(
	rules []resource.HeaderTransformationRule,
) []*adminv1.HeaderTransformationRule {
	responses := make([]*adminv1.HeaderTransformationRule, len(rules))
	for i, rule := range rules {
		responses[i] = &adminv1.HeaderTransformationRule{
			Operation: headerTransformationOperationResponse(rule.Operation),
			Name:      rule.Name,
			Value:     rule.Value,
		}
	}
	return responses
}

func headerTransformationOperationResponse(
	operation resource.HeaderTransformationOperation,
) adminv1.HeaderTransformationOperation {
	switch operation {
	case resource.HeaderTransformationRemove:
		return adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_REMOVE
	case resource.HeaderTransformationRename:
		return adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_RENAME
	case resource.HeaderTransformationReplace:
		return adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_REPLACE
	case resource.HeaderTransformationAdd:
		return adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_ADD
	case resource.HeaderTransformationAppend:
		return adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_APPEND
	default:
		return adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_UNSPECIFIED
	}
}
