package tokenquota

import (
	"strconv"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func newTokenQuotaPolicyReply(policy *resource.TokenQuotaPolicy, names biz.PolicyTargetNames) *adminv1.TokenQuotaPolicy {
	status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := status.State == biz.ResourceStateDisabled
	reply := &adminv1.TokenQuotaPolicy{
		Id: policy.Name, Version: strconv.FormatInt(policy.Generation, 10), Status: adminservice.NewResourceStatus(status),
		Name: policy.Spec.DisplayName, Description: policy.Spec.Description, Enabled: policy.Spec.Enabled,
		Targets: adminservice.NewPolicyTargets(policy.Generation, disabled, policy.Spec.TargetRefs, policy.Status.Targets, names),
		Subject: &adminv1.TokenQuotaSubject{
			Type: string(policy.Spec.Subject.Type), HeaderName: policy.Spec.Subject.HeaderName,
		},
		Quota: &adminv1.TokenQuota{
			Tokens: policy.Spec.Quota.Tokens, WindowSeconds: policy.Spec.Quota.WindowSeconds,
		},
		FailurePolicy: string(policy.Spec.FailurePolicy),
		Response:      &adminv1.TokenQuotaResponse{Message: policy.Spec.Response.Message},
		CreatedAt:     adminservice.NewTimestamp(policy.CreationTimestamp.Time),
	}
	if reply.Response.Message == "" {
		reply.Response.Message = "Token quota exceeded"
	}
	return reply
}
