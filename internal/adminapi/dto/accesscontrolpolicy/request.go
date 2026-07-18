package accesscontrolpolicy

import (
	"errors"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Validate 校验创建 AccessControlPolicy 请求
func (r *CreateAccessControlPolicyReq) Validate() error {
	return r.AccessControlPolicyConfig.Validate()
}

// Validate 校验更新 AccessControlPolicy 请求
func (r *UpdateAccessControlPolicyReq) Validate() error {
	if r.Version == "" {
		return errors.New("版本不能为空")
	}
	return r.AccessControlPolicyConfig.Validate()
}

// Validate 校验启用状态请求
func (r *SetEnabledReq) Validate() error {
	return nil
}

// Validate 校验访问控制策略核心配置
func (c *AccessControlPolicyConfig) Validate() error {
	if c.Name == "" {
		return errors.New("名称不能为空")
	}
	switch c.DefaultAction {
	case "", resource.AccessControlActionAllow, resource.AccessControlActionDeny:
	default:
		return errors.New("默认动作不正确")
	}
	if len(c.Rules) == 0 && c.DefaultAction != resource.AccessControlActionDeny {
		return errors.New("至少需要一条访问控制规则，或将默认动作设置为拒绝")
	}
	for _, rule := range c.Rules {
		if rule.Name == "" {
			return errors.New("访问控制规则名称不能为空")
		}
		switch rule.Action {
		case resource.AccessControlActionAllow, resource.AccessControlActionDeny:
		default:
			return errors.New("访问控制规则动作不正确")
		}
		for _, condition := range rule.Conditions {
			if err := validateCondition(condition); err != nil {
				return err
			}
		}
	}
	if c.Response.StatusCode < 0 {
		return errors.New("拒绝响应状态码不能小于 0")
	}
	return nil
}

func validateCondition(condition resource.AccessControlCondition) error {
	if condition.Value == "" {
		return errors.New("访问控制条件值不能为空")
	}
	switch condition.Type {
	case resource.AccessControlConditionTypeIP,
		resource.AccessControlConditionTypeConsumer,
		resource.AccessControlConditionTypeTenant:
		return nil
	case resource.AccessControlConditionTypeHeader:
		if condition.Name == "" {
			return errors.New("Header 访问控制条件必须填写名称")
		}
		return nil
	default:
		return errors.New("访问控制条件类型不正确")
	}
}
