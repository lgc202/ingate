package headertransformation

import (
	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	policybiz "github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func headerTransformationPolicyResponse(
	policy *resource.HeaderTransformationPolicy,
	names policybiz.TargetNames,
) *adminv1.HeaderTransformationPolicy {
	status := policybiz.Status(
		policy.Generation,
		policy.Spec.Enabled,
		len(policy.Spec.TargetRefs),
		policy.Status.Conditions,
	)
	disabled := status.State == resourceview.StateDisabled
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
	return lo.Map(rules, func(rule resource.HeaderTransformationRule, _ int) *adminv1.HeaderTransformationRule {
		return &adminv1.HeaderTransformationRule{
			Operation: headerTransformationOperationResponse(rule.Operation),
			Name:      rule.Name,
			Value:     rule.Value,
		}
	})
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
