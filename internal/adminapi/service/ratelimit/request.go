package ratelimit

import (
	"strings"

	"golang.org/x/net/http/httpguts"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func parseRateLimitPolicySpec(
	displayName string,
	enabled bool,
	targetConfigs []*adminv1.PolicyTargetRef,
	subjectConfig *adminv1.RateLimitSubject,
	limitConfig *adminv1.RateLimit,
) (resource.RateLimitPolicySpec, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return resource.RateLimitPolicySpec{}, adminv1.ErrorInvalidArgument("限流策略名称不能为空")
	}
	targets, err := adminservice.PolicyTargetRefs(
		targetConfigs,
		resource.KindGateway,
		resource.KindRoute,
	)
	if err != nil {
		return resource.RateLimitPolicySpec{}, err
	}
	subject, err := parseRateLimitSubject(subjectConfig)
	if err != nil {
		return resource.RateLimitPolicySpec{}, err
	}
	limit, err := parseRateLimit(limitConfig)
	if err != nil {
		return resource.RateLimitPolicySpec{}, err
	}

	return resource.RateLimitPolicySpec{
		DisplayName: displayName,
		Enabled:     enabled,
		TargetRefs:  targets,
		Subject:     subject,
		Limit:       limit,
	}, nil
}

func parseRateLimitSubject(config *adminv1.RateLimitSubject) (resource.RateLimitSubject, error) {
	if config == nil {
		return resource.RateLimitSubject{}, adminv1.ErrorInvalidArgument("限流计数对象不能为空")
	}

	headerName := strings.ToLower(strings.TrimSpace(config.GetHeaderName()))
	switch config.GetType() {
	case adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_SHARED:
		if headerName != "" {
			return resource.RateLimitSubject{}, adminv1.ErrorInvalidArgument("共享计数不能配置请求头名称")
		}
		return resource.RateLimitSubject{Type: resource.RateLimitSubjectShared}, nil
	case adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_IP:
		if headerName != "" {
			return resource.RateLimitSubject{}, adminv1.ErrorInvalidArgument("按客户端 IP 计数不能配置请求头名称")
		}
		return resource.RateLimitSubject{Type: resource.RateLimitSubjectIP}, nil
	case adminv1.RateLimitSubjectType_RATE_LIMIT_SUBJECT_TYPE_HEADER:
		if !httpguts.ValidHeaderFieldName(headerName) {
			return resource.RateLimitSubject{}, adminv1.ErrorInvalidArgument("限流请求头名称不正确")
		}
		return resource.RateLimitSubject{
			Type:       resource.RateLimitSubjectHeader,
			HeaderName: headerName,
		}, nil
	default:
		return resource.RateLimitSubject{}, adminv1.ErrorInvalidArgument("限流计数对象不正确")
	}
}

func parseRateLimit(config *adminv1.RateLimit) (resource.RateLimit, error) {
	if config == nil {
		return resource.RateLimit{}, adminv1.ErrorInvalidArgument("限流额度不能为空")
	}
	requests := config.GetRequests()
	if requests < 1 || requests > resource.RateLimitMaxRequests {
		return resource.RateLimit{}, adminv1.ErrorInvalidArgument("限流请求数超出支持范围")
	}
	windowSeconds := config.GetWindowSeconds()
	if windowSeconds < 1 || windowSeconds > resource.RateLimitMaxWindowSeconds {
		return resource.RateLimit{}, adminv1.ErrorInvalidArgument("限流周期超出支持范围")
	}
	return resource.RateLimit{
		Requests:      requests,
		WindowSeconds: windowSeconds,
	}, nil
}
