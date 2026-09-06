package mockresponse

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
	policy *resource.MockResponsePolicy,
	names policybiz.TargetNames,
) *adminv1.MockResponsePolicy {
	resourceStatus := policybiz.Status(
		policy.Generation,
		policy.Spec.Enabled,
		len(policy.Spec.TargetRefs),
		policy.Status.Conditions,
	)
	disabled := resourceStatus.State == status.StateDisabled
	return &adminv1.MockResponsePolicy{
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
		StatusCode:  policy.Spec.StatusCode,
		ContentType: policy.Spec.ContentType,
		Headers:     headerResponses(policy.Spec.Headers),
		Body:        policy.Spec.Body,
		State:       conversion.ResourceState(resourceStatus.State),
		Message:     conversion.ResourceMessage(resourceStatus.Reason),
		Version:     policy.Generation,
		CreatedAt:   conversion.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt:   conversion.Timestamp(conversion.ResourceUpdatedAt(policy.Annotations)),
	}
}

func headerResponses(values []resource.HeaderValue) []*adminv1.MockResponseHeader {
	return lo.Map(values, func(value resource.HeaderValue, _ int) *adminv1.MockResponseHeader {
		return &adminv1.MockResponseHeader{Name: value.Name, Value: value.Value}
	})
}
