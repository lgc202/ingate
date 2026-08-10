package ratelimit

import (
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func buildRateLimitPolicySpec(
	name, description string,
	enabled bool,
	targets []*adminv1.PolicyTargetRef,
	rules []*adminv1.RateLimitRule,
	response *adminv1.RateLimitResponse,
	failurePolicy string,
) (resource.RateLimitPolicySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("名称不能为空")
	}
	refs, err := adminservice.BuildPolicyTargetRefs(targets)
	if err != nil {
		return resource.RateLimitPolicySpec{}, err
	}
	if len(rules) == 0 {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("至少需要一条限流规则")
	}
	spec := resource.RateLimitPolicySpec{
		DisplayName: name, Description: description, Enabled: enabled, TargetRefs: refs,
		FailurePolicy: resource.RateLimitFailurePolicy(failurePolicy),
	}
	if spec.FailurePolicy == "" {
		spec.FailurePolicy = resource.RateLimitFailurePolicyFailOpen
	}
	if spec.FailurePolicy != resource.RateLimitFailurePolicyFailOpen && spec.FailurePolicy != resource.RateLimitFailurePolicyFailClose {
		return resource.RateLimitPolicySpec{}, adminservice.BadRequest("失败策略不正确")
	}
	seen := make(map[string]struct{}, len(rules))
	for _, input := range rules {
		if input == nil || strings.TrimSpace(input.GetName()) == "" || input.GetKey() == nil || input.GetLimit() == nil {
			return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流规则配置不完整")
		}
		ruleName := strings.TrimSpace(input.GetName())
		if _, exists := seen[ruleName]; exists {
			return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流规则名称不能重复")
		}
		seen[ruleName] = struct{}{}
		limit := input.GetLimit()
		if limit.GetRequests() <= 0 || limit.GetWindowSeconds() <= 0 || limit.GetBurst() < 0 {
			return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流额度必须大于 0")
		}
		rule := resource.RateLimitRule{Name: ruleName, Limit: resource.RateLimitQuota{
			Requests: int(limit.GetRequests()), WindowSeconds: int(limit.GetWindowSeconds()), Burst: int(limit.GetBurst()),
		}}
		if len(input.GetKey().GetParts()) == 0 {
			return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流规则必须配置计数维度")
		}
		for _, inputPart := range input.GetKey().GetParts() {
			if inputPart == nil {
				return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流维度不能为空")
			}
			part := resource.RateLimitKeyPart{Type: resource.RateLimitKeyType(inputPart.GetType()), Name: strings.TrimSpace(inputPart.GetName())}
			switch part.Type {
			case resource.RateLimitKeyTypeIP, resource.RateLimitKeyTypeRoute, resource.RateLimitKeyTypeGateway:
				part.Name = ""
			case resource.RateLimitKeyTypeHeader:
				if part.Name == "" {
					return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流维度名称不能为空")
				}
				if len(k8svalidation.IsHTTPHeaderName(part.Name)) > 0 {
					return resource.RateLimitPolicySpec{}, adminservice.BadRequest("请求头名称不正确")
				}
			case resource.RateLimitKeyTypeQuery, resource.RateLimitKeyTypeCookie:
				if part.Name == "" {
					return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流维度名称不能为空")
				}
			default:
				return resource.RateLimitPolicySpec{}, adminservice.BadRequest("限流维度不正确")
			}
			rule.Key.Parts = append(rule.Key.Parts, part)
		}
		spec.Rules = append(spec.Rules, rule)
	}
	if response != nil {
		if response.GetStatusCode() != 0 && (response.GetStatusCode() < 400 || response.GetStatusCode() > 599) {
			return resource.RateLimitPolicySpec{}, adminservice.BadRequest("超限响应状态码必须在 400 到 599 之间")
		}
		spec.Response = resource.RateLimitResponse{
			StatusCode: int(response.GetStatusCode()), Message: response.GetMessage(),
			QuotaHeaderEnabled: response.GetQuotaHeaderEnabled(),
		}
	}
	return spec, nil
}
