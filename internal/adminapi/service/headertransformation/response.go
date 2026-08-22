package headertransformation

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func policyResponse(
	policy *resource.HeaderTransformationPolicy,
	names biz.PolicyTargetNames,
) *adminv1.HeaderTransformationPolicy {
	status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
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
		RequestRules:  ruleResponses(policy.Spec.RequestRules),
		ResponseRules: ruleResponses(policy.Spec.ResponseRules),
		State:         adminservice.ResourceState(status.State),
		Message:       adminservice.ResourceMessage(status.Reason),
		Version:       policy.Generation,
		CreatedAt:     adminservice.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt:     adminservice.Timestamp(adminservice.ResourceUpdatedAt(policy.Annotations)),
	}
}

func ruleResponses(rules []resource.HeaderTransformationRule) []*adminv1.HeaderTransformationRule {
	result := make([]*adminv1.HeaderTransformationRule, 0, len(rules))
	for _, rule := range rules {
		result = append(result, &adminv1.HeaderTransformationRule{
			Operation: operationResponse(rule.Operation),
			Name:      rule.Name,
			Value:     rule.Value,
		})
	}
	return result
}

func operationResponse(value resource.HeaderTransformationOperation) adminv1.HeaderTransformationOperation {
	switch value {
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
