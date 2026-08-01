package accesscontrolpolicy

import (
	admindto "github.com/lgc202/ingate/internal/admin/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Spec 将已校验的创建请求转换为声明式 AccessControlPolicySpec
func (r CreateAccessControlPolicyReq) Spec() resource.AccessControlPolicySpec {
	return r.AccessControlPolicyConfig.spec()
}

// Spec 将已校验的更新请求转换为声明式 AccessControlPolicySpec
func (r UpdateAccessControlPolicyReq) Spec() resource.AccessControlPolicySpec {
	return r.AccessControlPolicyConfig.spec()
}

func (c AccessControlPolicyConfig) spec() resource.AccessControlPolicySpec {
	return resource.AccessControlPolicySpec{
		DisplayName:   c.Name,
		Description:   c.Description,
		Enabled:       c.Enabled,
		TargetRefs:    targetRefsFromRequest(c.Targets),
		DefaultAction: resource.AccessControlAction(c.DefaultAction),
		Rules:         rulesFromRequest(c.Rules),
		Response: resource.AccessControlDenyResponse{
			StatusCode: c.Response.StatusCode,
			Message:    c.Response.Message,
		},
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

func rulesFromRequest(items []Rule) []resource.AccessControlRule {
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
