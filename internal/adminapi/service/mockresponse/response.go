package mockresponse

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func policyResponse(policy *resource.MockResponsePolicy, names biz.PolicyTargetNames) *adminv1.MockResponsePolicy {
	status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := status.State == biz.ResourceStateDisabled
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
		Headers:     headerResponses(policy.Spec.Headers),
		Body:        policy.Spec.Body,
		State:       adminservice.ResourceState(status.State),
		Message:     adminservice.ResourceMessage(status.Reason),
		Version:     policy.Generation,
		CreatedAt:   adminservice.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt:   adminservice.Timestamp(adminservice.ResourceUpdatedAt(policy.Annotations)),
	}
}

func headerResponses(values []resource.HeaderValue) []*adminv1.MockResponseHeader {
	headers := make([]*adminv1.MockResponseHeader, 0, len(values))
	for _, value := range values {
		headers = append(headers, &adminv1.MockResponseHeader{Name: value.Name, Value: value.Value})
	}
	return headers
}
