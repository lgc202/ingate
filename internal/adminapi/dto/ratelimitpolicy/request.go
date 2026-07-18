package ratelimitpolicy

import (
	"errors"
	"fmt"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
)

const (
	maxPluginInteger      = 1<<31 - 1
	minResponseStatusCode = 400
	maxResponseStatusCode = 599
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
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("名称不能为空")
	}
	if err := admindto.ValidatePolicyTargets(c.Targets); err != nil {
		return err
	}
	if len(c.Rules) == 0 {
		return errors.New("至少需要一条限流规则")
	}

	seenRules := make(map[string]struct{}, len(c.Rules))
	for i := range c.Rules {
		rule := &c.Rules[i]
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" {
			return errors.New("限流规则名称不能为空")
		}
		if _, exists := seenRules[rule.Name]; exists {
			return fmt.Errorf("限流规则名称 %q 重复", rule.Name)
		}
		seenRules[rule.Name] = struct{}{}
		if rule.Limit.Requests <= 0 {
			return errors.New("限流请求数必须大于 0")
		}
		if rule.Limit.Requests > maxPluginInteger {
			return errors.New("限流请求数超出支持范围")
		}
		if rule.Limit.WindowSeconds <= 0 {
			return errors.New("限流窗口必须大于 0")
		}
		if rule.Limit.WindowSeconds > maxPluginInteger {
			return errors.New("限流窗口超出支持范围")
		}
		if rule.Limit.Burst < 0 {
			return errors.New("限流突发容量不能小于 0")
		}
		if rule.Limit.Burst > maxPluginInteger {
			return errors.New("限流突发容量超出支持范围")
		}
		if len(rule.Key.Parts) == 0 {
			return errors.New("限流规则必须配置计数维度")
		}
		for j := range rule.Key.Parts {
			if err := rule.Key.Parts[j].Validate(); err != nil {
				return err
			}
		}
	}
	if c.Response.StatusCode != 0 &&
		(c.Response.StatusCode < minResponseStatusCode || c.Response.StatusCode > maxResponseStatusCode) {
		return errors.New("超限响应状态码必须在 400 到 599 之间")
	}
	switch c.FailurePolicy {
	case "", FailurePolicyFailOpen, FailurePolicyFailClose:
	default:
		return errors.New("失败策略不正确")
	}
	return nil
}

// Validate 校验限流计数维度
func (p *KeyPart) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	switch p.Type {
	case KeyTypeIP,
		KeyTypeRoute,
		KeyTypeGateway,
		KeyTypeRouteRule:
		p.Name = ""
		return nil
	case KeyTypeHeader:
		if p.Name == "" {
			return errors.New("请求头限流维度必须填写名称")
		}
		if len(k8svalidation.IsHTTPHeaderName(p.Name)) > 0 {
			return errors.New("请求头名称不正确")
		}
		return nil
	case KeyTypeQuery, KeyTypeCookie:
		if p.Name == "" {
			return errors.New("查询参数或 Cookie 限流维度必须填写名称")
		}
		return nil
	default:
		return errors.New("限流维度不正确")
	}
}
