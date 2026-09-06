package headertransformation

import (
	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	policybiz "github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	"github.com/lgc202/ingate/internal/adminapi/service/conversion"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/target"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func policyResponse(
	policy *resource.HeaderTransformationPolicy,
	names policybiz.TargetNames,
) *adminv1.HeaderTransformationPolicy {
	resourceStatus := policybiz.Status(
		policy.Generation,
		policy.Spec.Enabled,
		len(policy.Spec.TargetRefs),
		policy.Status.Conditions,
	)
	disabled := resourceStatus.State == status.StateDisabled
	return &adminv1.HeaderTransformationPolicy{
		Id:      policy.Name,
		Name:    policy.Spec.DisplayName,
		Enabled: policy.Spec.Enabled,
		Targets: target.Responses(
			policy.Generation,
			disabled,
			policy.Spec.TargetRefs,
			policy.Status.Targets,
			names,
		),
		RequestRules:  ruleResponses(policy.Spec.RequestRules),
		ResponseRules: ruleResponses(policy.Spec.ResponseRules),
		State:         conversion.ResourceState(resourceStatus.State),
		Message:       conversion.ResourceMessage(resourceStatus.Reason),
		Version:       policy.Generation,
		CreatedAt:     conversion.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt:     conversion.Timestamp(conversion.ResourceUpdatedAt(policy.Annotations)),
	}
}

func ruleResponses(
	rules []resource.HeaderTransformationRule,
) []*adminv1.HeaderTransformationRule {
	return lo.Map(rules, func(rule resource.HeaderTransformationRule, _ int) *adminv1.HeaderTransformationRule {
		return &adminv1.HeaderTransformationRule{
			Operation: operationResponse(rule.Operation),
			Name:      rule.Name,
			Value:     rule.Value,
		}
	})
}

func operationResponse(
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
