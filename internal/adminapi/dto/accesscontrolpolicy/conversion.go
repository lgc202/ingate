package accesscontrolpolicy

import (
	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	accesscontrolpolicyservice "github.com/lgc202/ingate/internal/adminapi/service/accesscontrolpolicy"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewCreatePolicyParams 转换创建请求为访问控制策略用例参数
func NewCreatePolicyParams(request CreateAccessControlPolicyReq) accesscontrolpolicyservice.CreatePolicyParams {
	return accesscontrolpolicyservice.CreatePolicyParams{PolicyParams: policyParams(request.AccessControlPolicyConfig)}
}

// NewUpdatePolicyParams 转换更新请求为访问控制策略用例参数
func NewUpdatePolicyParams(request UpdateAccessControlPolicyReq) accesscontrolpolicyservice.UpdatePolicyParams {
	return accesscontrolpolicyservice.UpdatePolicyParams{
		Version:      request.Version,
		PolicyParams: policyParams(request.AccessControlPolicyConfig),
	}
}

func policyParams(config AccessControlPolicyConfig) accesscontrolpolicyservice.PolicyParams {
	return accesscontrolpolicyservice.PolicyParams{
		Name:          config.Name,
		Description:   config.Description,
		Enabled:       config.Enabled,
		Targets:       targetParams(config.Targets),
		DefaultAction: resource.AccessControlAction(config.DefaultAction),
		Rules:         rulesToResource(config.Rules),
		Response: resource.AccessControlDenyResponse{
			StatusCode: config.Response.StatusCode,
			Message:    config.Response.Message,
		},
	}
}

func targetParams(targets []admindto.PolicyTargetReq) []accesscontrolpolicyservice.TargetParams {
	params := make([]accesscontrolpolicyservice.TargetParams, 0, len(targets))
	for _, target := range targets {
		params = append(params, accesscontrolpolicyservice.TargetParams{
			Kind: resource.Kind(target.Kind),
			ID:   target.ID,
		})
	}
	return params
}

func rulesToResource(items []Rule) []resource.AccessControlRule {
	rules := make([]resource.AccessControlRule, 0, len(items))
	for _, item := range items {
		conditions := make([]resource.AccessControlCondition, 0, len(item.Conditions))
		for _, condition := range item.Conditions {
			conditions = append(conditions, resource.AccessControlCondition{
				Type:  resource.AccessControlConditionType(condition.Type),
				Name:  condition.Name,
				Value: condition.Value,
			})
		}
		rules = append(rules, resource.AccessControlRule{
			Name:       item.Name,
			Action:     resource.AccessControlAction(item.Action),
			Conditions: conditions,
		})
	}
	return rules
}
