package accesscontrol

import (
	"net/netip"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func buildAccessControlPolicySpec(
	name, description string,
	enabled bool,
	targets []*adminv1.PolicyTargetRef,
	defaultAction string,
	rules []*adminv1.AccessControlRule,
	response *adminv1.AccessControlDenyResponse,
) (resource.AccessControlPolicySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.AccessControlPolicySpec{}, adminservice.BadRequest("名称不能为空")
	}
	refs, err := adminservice.BuildPolicyTargetRefs(targets)
	if err != nil {
		return resource.AccessControlPolicySpec{}, err
	}
	action := resource.AccessControlAction(defaultAction)
	if action == "" {
		action = resource.AccessControlActionAllow
	}
	if action != resource.AccessControlActionAllow && action != resource.AccessControlActionDeny {
		return resource.AccessControlPolicySpec{}, adminservice.BadRequest("默认动作不正确")
	}
	if len(rules) == 0 && action != resource.AccessControlActionDeny {
		return resource.AccessControlPolicySpec{}, adminservice.BadRequest("至少需要一条访问控制规则，或将默认动作设置为拒绝")
	}
	spec := resource.AccessControlPolicySpec{
		DisplayName: name, Description: description, Enabled: enabled, TargetRefs: refs, DefaultAction: action,
	}
	seen := make(map[string]struct{}, len(rules))
	for _, input := range rules {
		if input == nil || strings.TrimSpace(input.GetName()) == "" {
			return resource.AccessControlPolicySpec{}, adminservice.BadRequest("访问控制规则名称不能为空")
		}
		rule := resource.AccessControlRule{Name: strings.TrimSpace(input.GetName()), Action: resource.AccessControlAction(input.GetAction())}
		if _, exists := seen[rule.Name]; exists {
			return resource.AccessControlPolicySpec{}, adminservice.BadRequest("访问控制规则名称不能重复")
		}
		seen[rule.Name] = struct{}{}
		if rule.Action != resource.AccessControlActionAllow && rule.Action != resource.AccessControlActionDeny {
			return resource.AccessControlPolicySpec{}, adminservice.BadRequest("访问控制规则动作不正确")
		}
		for _, inputCondition := range input.GetConditions() {
			if inputCondition == nil || strings.TrimSpace(inputCondition.GetValue()) == "" {
				return resource.AccessControlPolicySpec{}, adminservice.BadRequest("访问控制条件值不能为空")
			}
			condition := resource.AccessControlCondition{
				Type:  resource.AccessControlConditionType(inputCondition.GetType()),
				Name:  strings.TrimSpace(inputCondition.GetName()),
				Value: strings.TrimSpace(inputCondition.GetValue()),
			}
			switch condition.Type {
			case resource.AccessControlConditionTypeIP:
				condition.Name = ""
				if _, err := netip.ParseAddr(condition.Value); err != nil {
					if _, err := netip.ParsePrefix(condition.Value); err != nil {
						return resource.AccessControlPolicySpec{}, adminservice.BadRequest("客户端 IP 必须是 IP 地址或 CIDR")
					}
				}
			case resource.AccessControlConditionTypeHeader:
				if condition.Name == "" {
					return resource.AccessControlPolicySpec{}, adminservice.BadRequest("请求头访问控制条件必须填写名称")
				}
				if len(k8svalidation.IsHTTPHeaderName(condition.Name)) > 0 {
					return resource.AccessControlPolicySpec{}, adminservice.BadRequest("请求头名称不正确")
				}
			default:
				return resource.AccessControlPolicySpec{}, adminservice.BadRequest("访问控制条件类型不正确")
			}
			rule.Conditions = append(rule.Conditions, condition)
		}
		spec.Rules = append(spec.Rules, rule)
	}
	if response != nil {
		if response.GetStatusCode() != 0 && (response.GetStatusCode() < 400 || response.GetStatusCode() > 599) {
			return resource.AccessControlPolicySpec{}, adminservice.BadRequest("拒绝响应状态码必须在 400 到 599 之间")
		}
		spec.Response = resource.AccessControlDenyResponse{
			StatusCode: int(response.GetStatusCode()), Message: response.GetMessage(),
		}
	}
	return spec, nil
}
