package ratelimitpolicy

import (
	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	ratelimitpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/ratelimitpolicy"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewCreatePolicyParams 转换创建请求为限流策略用例参数
func NewCreatePolicyParams(request CreateRateLimitPolicyReq) ratelimitpolicyservice.CreatePolicyParams {
	return ratelimitpolicyservice.CreatePolicyParams{PolicyParams: policyParams(request.RateLimitPolicyConfig)}
}

// NewUpdatePolicyParams 转换更新请求为限流策略用例参数
func NewUpdatePolicyParams(request UpdateRateLimitPolicyReq) ratelimitpolicyservice.UpdatePolicyParams {
	return ratelimitpolicyservice.UpdatePolicyParams{
		Version:      request.Version,
		PolicyParams: policyParams(request.RateLimitPolicyConfig),
	}
}

func policyParams(config RateLimitPolicyConfig) ratelimitpolicyservice.PolicyParams {
	return ratelimitpolicyservice.PolicyParams{
		Name:        config.Name,
		Description: config.Description,
		Enabled:     config.Enabled,
		Targets:     targetParams(config.Targets),
		Rules:       rulesToResource(config.Rules),
		Response: resource.RateLimitResponse{
			StatusCode:         config.Response.StatusCode,
			Message:            config.Response.Message,
			QuotaHeaderEnabled: config.Response.QuotaHeaderEnabled,
		},
		FailurePolicy: resource.RateLimitFailurePolicy(config.FailurePolicy),
	}
}

func targetParams(targets []admindto.PolicyTargetReq) []ratelimitpolicyservice.TargetParams {
	params := make([]ratelimitpolicyservice.TargetParams, 0, len(targets))
	for _, target := range targets {
		params = append(params, ratelimitpolicyservice.TargetParams{
			Kind: resource.Kind(target.Kind),
			ID:   target.ID,
		})
	}
	return params
}

func rulesToResource(items []Rule) []resource.RateLimitRule {
	rules := make([]resource.RateLimitRule, 0, len(items))
	for _, item := range items {
		parts := make([]resource.RateLimitKeyPart, 0, len(item.Key.Parts))
		for _, part := range item.Key.Parts {
			parts = append(parts, resource.RateLimitKeyPart{
				Type: resource.RateLimitKeyType(part.Type),
				Name: part.Name,
			})
		}
		rules = append(rules, resource.RateLimitRule{
			Name: item.Name,
			Key:  resource.RateLimitKey{Parts: parts},
			Limit: resource.RateLimitQuota{
				Requests:      item.Limit.Requests,
				WindowSeconds: item.Limit.WindowSeconds,
				Burst:         item.Limit.Burst,
			},
		})
	}
	return rules
}
