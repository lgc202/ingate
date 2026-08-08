package ratelimit

import (
	"strconv"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func newRateLimitPolicyReply(policy *resource.RateLimitPolicy, names biz.PolicyTargetNames) *adminv1.RateLimitPolicy {
	status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := status.State == biz.ResourceStateDisabled
	reply := &adminv1.RateLimitPolicy{
		Id: policy.Name, Version: strconv.FormatInt(policy.Generation, 10), Status: adminservice.NewResourceStatus(status),
		Name: policy.Spec.DisplayName, Description: policy.Spec.Description, Enabled: policy.Spec.Enabled,
		Targets:       adminservice.NewPolicyTargets(policy.Generation, disabled, policy.Spec.TargetRefs, policy.Status.Targets, names),
		FailurePolicy: string(policy.Spec.FailurePolicy), CreatedAt: adminservice.NewTimestamp(policy.CreationTimestamp.Time),
		Response: &adminv1.RateLimitResponse{
			StatusCode: int32(policy.Spec.Response.StatusCode), Message: policy.Spec.Response.Message,
			QuotaHeaderEnabled: policy.Spec.Response.QuotaHeaderEnabled,
		},
	}
	if reply.FailurePolicy == "" {
		reply.FailurePolicy = string(resource.RateLimitFailurePolicyFailOpen)
	}
	if reply.Response.StatusCode == 0 {
		reply.Response.StatusCode = 429
	}
	if reply.Response.Message == "" {
		reply.Response.Message = "Too many requests"
	}
	for _, item := range policy.Spec.Rules {
		rule := &adminv1.RateLimitRule{Name: item.Name, Key: &adminv1.RateLimitKey{}, Limit: &adminv1.RateLimitQuota{
			Requests: int32(item.Limit.Requests), WindowSeconds: int32(item.Limit.WindowSeconds), Burst: int32(item.Limit.Burst),
		}}
		for _, part := range item.Key.Parts {
			rule.Key.Parts = append(rule.Key.Parts, &adminv1.RateLimitKeyPart{Type: string(part.Type), Name: part.Name})
		}
		reply.Rules = append(reply.Rules, rule)
	}
	return reply
}
