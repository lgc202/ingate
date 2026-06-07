package dto

import (
	"errors"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Validate 校验创建 RateLimitPolicy 请求
func (r *CreateRateLimitPolicyReq) Validate() error {
	return r.RateLimitPolicyConfig.Validate()
}

// Validate 校验更新 RateLimitPolicy 请求
func (r *UpdateRateLimitPolicyReq) Validate() error {
	if r.Version == "" {
		return errors.New("版本不能为空")
	}
	return r.RateLimitPolicyConfig.Validate()
}

// Validate 校验启用状态请求
func (r *SetEnabledReq) Validate() error {
	return nil
}

// Validate 校验限流策略核心配置
func (c *RateLimitPolicyConfig) Validate() error {
	if c.Name == "" {
		return errors.New("名称不能为空")
	}
	switch c.Mode {
	case resource.RateLimitModeLocal:
		if c.Global != nil {
			return errors.New("本地限流不能配置 Redis")
		}
	case resource.RateLimitModeGlobal:
		if c.Global == nil || c.Global.RedisRef == "" {
			return errors.New("全局限流必须选择 Redis 配置")
		}
	default:
		return errors.New("限流模式不正确")
	}
	if len(c.Rules) == 0 {
		return errors.New("至少需要一条限流规则")
	}
	for _, rule := range c.Rules {
		if rule.Name == "" {
			return errors.New("限流规则名称不能为空")
		}
		if rule.Limit.Requests <= 0 {
			return errors.New("限流请求数必须大于 0")
		}
		if rule.Limit.WindowSeconds <= 0 {
			return errors.New("限流窗口必须大于 0")
		}
		if len(rule.Key.Parts) == 0 {
			return errors.New("限流规则必须配置计数维度")
		}
		for _, part := range rule.Key.Parts {
			if err := validateKeyPart(part); err != nil {
				return err
			}
		}
	}
	if c.Response.StatusCode < 0 {
		return errors.New("超限响应状态码不能小于 0")
	}
	switch c.FailurePolicy {
	case "", resource.RateLimitFailurePolicyFailOpen, resource.RateLimitFailurePolicyFailClose:
	default:
		return errors.New("失败策略不正确")
	}
	return nil
}

func validateKeyPart(part resource.RateLimitKeyPart) error {
	switch part.Type {
	case resource.RateLimitKeyTypeIP, resource.RateLimitKeyTypeConsumer, resource.RateLimitKeyTypeRoute, resource.RateLimitKeyTypeGateway:
		return nil
	case resource.RateLimitKeyTypeHeader, resource.RateLimitKeyTypeQuery, resource.RateLimitKeyTypeCookie:
		if part.Name == "" {
			return errors.New("Header、Query、Cookie 限流维度必须填写名称")
		}
		return nil
	default:
		return errors.New("限流维度不正确")
	}
}
