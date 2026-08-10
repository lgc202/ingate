package ratelimit

import (
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func buildRateLimitPolicySpec(
	name string,
	enabled bool,
	targets []*adminv1.PolicyTargetRef,
	subject *adminv1.RateLimitSubject,
	limit *adminv1.RateLimit,
) (resource.RateLimitPolicySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流策略名称不能为空")
	}
	refs, err := adminservice.BuildPolicyTargetRefs(targets)
	if err != nil {
		return resource.RateLimitPolicySpec{}, err
	}
	if subject == nil {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("请选择限流计数对象")
	}
	if limit == nil {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("请配置限流额度")
	}

	result := resource.RateLimitPolicySpec{
		DisplayName: name,
		Enabled:     enabled,
		TargetRefs:  refs,
		Limit: resource.RateLimit{
			Requests:      limit.GetRequests(),
			WindowSeconds: limit.GetWindowSeconds(),
		},
	}
	if result.Limit.Requests <= 0 || result.Limit.Requests > resource.RateLimitMaxRequests {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流请求数不正确")
	}
	if result.Limit.WindowSeconds <= 0 || result.Limit.WindowSeconds > resource.RateLimitMaxWindowSeconds {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流时间窗口不正确")
	}

	headerName := strings.ToLower(strings.TrimSpace(subject.GetHeaderName()))
	switch subject.GetType() {
	case adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_SHARED:
		result.Subject.Type = resource.RateLimitSubjectShared
	case adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_IP:
		result.Subject.Type = resource.RateLimitSubjectIP
	case adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_HEADER:
		if headerName == "" || len(k8svalidation.IsHTTPHeaderName(headerName)) > 0 {
			return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流请求头名称不正确")
		}
		result.Subject.Type = resource.RateLimitSubjectHeader
		result.Subject.HeaderName = headerName
	default:
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流计数对象不正确")
	}
	if result.Subject.Type != resource.RateLimitSubjectHeader && headerName != "" {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("只有按请求头计数时才能填写请求头名称")
	}
	return result, nil
}
