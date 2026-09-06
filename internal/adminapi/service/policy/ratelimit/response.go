package ratelimit

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	policybiz "github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	"github.com/lgc202/ingate/internal/adminapi/service/conversion"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/target"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func policyResponse(policy *resource.RateLimitPolicy, names policybiz.TargetNames) *adminv1.RateLimitPolicy {
	resourceStatus := policybiz.Status(
		policy.Generation,
		policy.Spec.Enabled,
		len(policy.Spec.TargetRefs),
		policy.Status.Conditions,
	)
	disabled := resourceStatus.State == status.StateDisabled
	return &adminv1.RateLimitPolicy{
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
		Subject: &adminv1.RateLimitSubject{
			Type:       subjectTypeResponse(policy.Spec.Subject.Type),
			HeaderName: policy.Spec.Subject.HeaderName,
		},
		Limit: &adminv1.RateLimit{
			Requests:      policy.Spec.Limit.Requests,
			WindowSeconds: policy.Spec.Limit.WindowSeconds,
		},
		State:     conversion.ResourceState(resourceStatus.State),
		Message:   conversion.ResourceMessage(resourceStatus.Reason),
		Version:   policy.Generation,
		CreatedAt: conversion.Timestamp(policy.CreationTimestamp.Time),
		UpdatedAt: conversion.Timestamp(conversion.ResourceUpdatedAt(policy.Annotations)),
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
