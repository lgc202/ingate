package ratelimit

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func rateLimitPolicyResponse(policy *resource.RateLimitPolicy, names biz.PolicyTargetNames) *adminv1.RateLimitPolicy {
	status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := status.State == biz.ResourceStateDisabled
	return &adminv1.RateLimitPolicy{
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
		Subject: &adminv1.RateLimitSubject{
			Type:       subjectTypeResponse(policy.Spec.Subject.Type),
			HeaderName: policy.Spec.Subject.HeaderName,
		},
		Limit: &adminv1.RateLimit{
			Requests:      policy.Spec.Limit.Requests,
			WindowSeconds: policy.Spec.Limit.WindowSeconds,
		},
		State:     adminservice.ResourceState(status.State),
		Message:   adminservice.ResourceMessage(status.Reason),
		Version:   policy.Generation,
		CreatedAt: adminservice.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt: adminservice.Timestamp(adminservice.ResourceUpdatedAt(policy.Annotations)),
	}
}

func subjectTypeResponse(subjectType resource.RateLimitSubjectType) adminv1.RateLimitSubjectType {
	switch subjectType {
	case resource.RateLimitSubjectShared:
		return adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_SHARED
	case resource.RateLimitSubjectIP:
		return adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_IP
	case resource.RateLimitSubjectHeader:
		return adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_HEADER
	default:
		return adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_UNSPECIFIED
	}
}
