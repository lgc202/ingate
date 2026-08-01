package accesscontrolpolicy

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	consoledto "github.com/lgc202/ingate/internal/console/dto"
)

const (
	minResponseStatusCode = 400
	maxResponseStatusCode = 599
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
	if r.Enabled == nil {
		return errors.New("启用状态不能为空")
	}
	return nil
}

// Value 返回已校验的启停值
func (r *SetEnabledReq) Value() bool {
	return *r.Enabled
}

// Validate 校验访问控制策略核心配置
func (c *AccessControlPolicyConfig) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("名称不能为空")
	}
	if err := consoledto.ValidatePolicyTargets(c.Targets); err != nil {
		return err
	}
	switch c.DefaultAction {
	case "", ActionAllow, ActionDeny:
	default:
		return errors.New("默认动作不正确")
	}
	if len(c.Rules) == 0 && c.DefaultAction != ActionDeny {
		return errors.New("至少需要一条访问控制规则，或将默认动作设置为拒绝")
	}

	seenRules := make(map[string]struct{}, len(c.Rules))
	for i := range c.Rules {
		rule := &c.Rules[i]
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" {
			return errors.New("访问控制规则名称不能为空")
		}
		if _, exists := seenRules[rule.Name]; exists {
			return fmt.Errorf("访问控制规则名称 %q 重复", rule.Name)
		}
		seenRules[rule.Name] = struct{}{}
		switch rule.Action {
		case ActionAllow, ActionDeny:
		default:
			return errors.New("访问控制规则动作不正确")
		}
		for j := range rule.Conditions {
			if err := rule.Conditions[j].Validate(); err != nil {
				return err
			}
		}
	}
	if c.Response.StatusCode != 0 &&
		(c.Response.StatusCode < minResponseStatusCode || c.Response.StatusCode > maxResponseStatusCode) {
		return errors.New("拒绝响应状态码必须在 400 到 599 之间")
	}
	return nil
}

// Validate 校验访问控制匹配条件
func (c *Condition) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	c.Value = strings.TrimSpace(c.Value)
	if c.Value == "" {
		return errors.New("访问控制条件值不能为空")
	}
	switch c.Type {
	case ConditionTypeIP:
		c.Name = ""
		if _, err := netip.ParseAddr(c.Value); err == nil {
			return nil
		}
		if _, err := netip.ParsePrefix(c.Value); err != nil {
			return errors.New("客户端 IP 必须是 IP 地址或 CIDR")
		}
		return nil
	case ConditionTypeHeader:
		if c.Name == "" {
			return errors.New("请求头访问控制条件必须填写名称")
		}
		if len(k8svalidation.IsHTTPHeaderName(c.Name)) > 0 {
			return errors.New("请求头名称不正确")
		}
		return nil
	default:
		return errors.New("访问控制条件类型不正确")
	}
}
