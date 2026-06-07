package dto

import (
	"errors"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Validate 校验创建 PolicyBinding 请求
func (r *CreatePolicyBindingReq) Validate() error {
	return r.PolicyBindingConfig.Validate()
}

// Validate 校验更新 PolicyBinding 请求
func (r *UpdatePolicyBindingReq) Validate() error {
	if r.Version == "" {
		return errors.New("版本不能为空")
	}
	return r.PolicyBindingConfig.Validate()
}

// Validate 校验启用状态请求
func (r *SetEnabledReq) Validate() error {
	return nil
}

// Validate 校验策略绑定核心配置
func (c *PolicyBindingConfig) Validate() error {
	if c.Name == "" {
		return errors.New("名称不能为空")
	}
	switch c.TargetRef.Kind {
	case resource.KindGateway, resource.KindRoute, resource.KindUpstream:
	default:
		return errors.New("绑定目标类型不正确")
	}
	if c.TargetRef.Name == "" {
		return errors.New("绑定目标不能为空")
	}
	if c.TargetRef.RuleName != "" && c.TargetRef.Kind != resource.KindRoute {
		return errors.New("只有 Route 绑定可以指定规则名称")
	}
	if len(c.Policies) == 0 {
		return errors.New("至少需要绑定一个策略")
	}
	for _, policy := range c.Policies {
		switch policy.Kind {
		case resource.KindAuthPolicy, resource.KindRateLimitPolicy:
		default:
			return errors.New("绑定策略类型不正确")
		}
		if policy.Name == "" {
			return errors.New("绑定策略不能为空")
		}
	}
	return nil
}
