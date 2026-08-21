package ratelimit

import (
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func createSpec(request *adminv1.CreateRateLimitPolicyRequest) (resource.RateLimitPolicySpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流策略名称不能为空")
	}
	targets, err := adminservice.PolicyTargetRefs(request.GetTargets())
	if err != nil {
		return resource.RateLimitPolicySpec{}, err
	}
	subject, err := rateLimitSubject(request.GetSubject())
	if err != nil {
		return resource.RateLimitPolicySpec{}, err
	}
	return resource.RateLimitPolicySpec{
		DisplayName: name,
		Enabled:     request.GetEnabled(),
		TargetRefs:  targets,
		Subject:     subject,
		Limit:       rateLimit(request.GetLimit()),
	}, nil
}

func updateSpec(request *adminv1.UpdateRateLimitPolicyRequest) (resource.RateLimitPolicySpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流策略名称不能为空")
	}
	targets, err := adminservice.PolicyTargetRefs(request.GetTargets())
	if err != nil {
		return resource.RateLimitPolicySpec{}, err
	}
	subject, err := rateLimitSubject(request.GetSubject())
	if err != nil {
		return resource.RateLimitPolicySpec{}, err
	}
	return resource.RateLimitPolicySpec{
		DisplayName: name,
		Enabled:     request.GetEnabled(),
		TargetRefs:  targets,
		Subject:     subject,
		Limit:       rateLimit(request.GetLimit()),
	}, nil
}

func rateLimit(input *adminv1.RateLimit) resource.RateLimit {
	return resource.RateLimit{
		Requests:      input.GetRequests(),
		WindowSeconds: input.GetWindowSeconds(),
	}
}

func rateLimitSubject(input *adminv1.RateLimitSubject) (resource.RateLimitSubject, error) {
	var subject resource.RateLimitSubject
	headerName := strings.ToLower(strings.TrimSpace(input.GetHeaderName()))
	switch input.GetType() {
	case adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_SHARED:
		subject.Type = resource.RateLimitSubjectShared
	case adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_IP:
		subject.Type = resource.RateLimitSubjectIP
	case adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_HEADER:
		if headerName == "" || len(k8svalidation.IsHTTPHeaderName(headerName)) > 0 {
			return resource.RateLimitSubject{}, adminservice.BadRequest("限流请求头名称不正确")
		}
		subject.Type = resource.RateLimitSubjectHeader
		subject.HeaderName = headerName
	default:
		return resource.RateLimitSubject{}, adminservice.BadRequest("限流计数对象不正确")
	}
	if subject.Type != resource.RateLimitSubjectHeader && headerName != "" {
		return resource.RateLimitSubject{}, adminservice.BadRequest("只有按请求头计数时才能填写请求头名称")
	}
	return subject, nil
}
