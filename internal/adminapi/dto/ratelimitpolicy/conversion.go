package ratelimitpolicy

import (
	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Spec 将已校验的创建请求转换为声明式 RateLimitPolicySpec
func (r CreateRateLimitPolicyReq) Spec() resource.RateLimitPolicySpec {
	return r.RateLimitPolicyConfig.spec()
}

// Spec 将已校验的更新请求转换为声明式 RateLimitPolicySpec
func (r UpdateRateLimitPolicyReq) Spec() resource.RateLimitPolicySpec {
	return r.RateLimitPolicyConfig.spec()
}

func (c RateLimitPolicyConfig) spec() resource.RateLimitPolicySpec {
	return resource.RateLimitPolicySpec{
		DisplayName: c.Name,
		Description: c.Description,
		Enabled:     c.Enabled,
		TargetRefs:  targetRefsFromRequest(c.Targets),
		Rules:       rulesFromRequest(c.Rules),
		Response: resource.RateLimitResponse{
			StatusCode:         c.Response.StatusCode,
			Message:            c.Response.Message,
			QuotaHeaderEnabled: c.Response.QuotaHeaderEnabled,
		},
		FailurePolicy: resource.RateLimitFailurePolicy(c.FailurePolicy),
	}
}

func targetRefsFromRequest(targets []admindto.PolicyTargetReq) []resource.PolicyTargetRef {
	refs := make([]resource.PolicyTargetRef, 0, len(targets))
	for _, target := range targets {
		refs = append(refs, resource.PolicyTargetRef{
			Kind: resource.Kind(target.Kind),
			Name: target.ID,
		})
	}
	return refs
}

func rulesFromRequest(items []Rule) []resource.RateLimitRule {
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
