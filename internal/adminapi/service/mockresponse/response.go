package mockresponse

import (
	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	policybiz "github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func mockResponsePolicyResponse(
	policy *resource.MockResponsePolicy,
	names policybiz.TargetNames,
) *adminv1.MockResponsePolicy {
	status := policybiz.Status(
		policy.Generation,
		policy.Spec.Enabled,
		len(policy.Spec.TargetRefs),
		policy.Status.Conditions,
	)
	disabled := status.State == resourceview.StateDisabled
	return &adminv1.MockResponsePolicy{
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
		StatusCode:  policy.Spec.StatusCode,
		ContentType: policy.Spec.ContentType,
		Headers:     mockResponseHeaderResponses(policy.Spec.Headers),
		Body:        policy.Spec.Body,
		State:       adminservice.ResourceState(status.State),
		Message:     adminservice.ResourceMessage(status.Reason),
		Version:     policy.Generation,
		CreatedAt:   adminservice.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt:   adminservice.Timestamp(adminservice.ResourceUpdatedAt(policy.Annotations)),
	}
}

func mockResponseHeaderResponses(values []resource.HeaderValue) []*adminv1.MockResponseHeader {
	return lo.Map(values, func(value resource.HeaderValue, _ int) *adminv1.MockResponseHeader {
		return &adminv1.MockResponseHeader{Name: value.Name, Value: value.Value}
	})
}
