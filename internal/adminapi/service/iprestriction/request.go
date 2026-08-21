package iprestriction

import (
	"maps"
	"net/netip"
	"slices"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func createSpec(request *adminv1.CreateIPRestrictionPolicyRequest) (resource.IPRestrictionPolicySpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.IPRestrictionPolicySpec{}, adminservice.BadRequest("IP 访问限制策略名称不能为空")
	}
	targets, err := adminservice.PolicyTargetRefs(request.GetTargets())
	if err != nil {
		return resource.IPRestrictionPolicySpec{}, err
	}
	allow, deny, err := ipRanges(request.GetAllow(), request.GetDeny())
	if err != nil {
		return resource.IPRestrictionPolicySpec{}, err
	}
	return resource.IPRestrictionPolicySpec{
		DisplayName: name,
		Enabled:     true,
		TargetRefs:  targets,
		Allow:       allow,
		Deny:        deny,
	}, nil
}

func updateSpec(request *adminv1.UpdateIPRestrictionPolicyRequest) (resource.IPRestrictionPolicySpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.IPRestrictionPolicySpec{}, adminservice.BadRequest("IP 访问限制策略名称不能为空")
	}
	targets, err := adminservice.PolicyTargetRefs(request.GetTargets())
	if err != nil {
		return resource.IPRestrictionPolicySpec{}, err
	}
	allow, deny, err := ipRanges(request.GetAllow(), request.GetDeny())
	if err != nil {
		return resource.IPRestrictionPolicySpec{}, err
	}
	return resource.IPRestrictionPolicySpec{
		DisplayName: name,
		Enabled:     request.GetEnabled(),
		TargetRefs:  targets,
		Allow:       allow,
		Deny:        deny,
	}, nil
}

func ipRanges(allow, deny []string) ([]string, []string, error) {
	if (len(allow) > 0) == (len(deny) > 0) {
		return nil, nil, adminservice.BadRequest("IP 允许列表和拒绝列表必须且只能配置一个")
	}

	normalizedAllow, err := normalizeIPRanges(allow)
	if err != nil {
		return nil, nil, adminservice.BadRequest("IP 允许列表包含无效的 IP 地址或 CIDR")
	}
	normalizedDeny, err := normalizeIPRanges(deny)
	if err != nil {
		return nil, nil, adminservice.BadRequest("IP 拒绝列表包含无效的 IP 地址或 CIDR")
	}
	return normalizedAllow, normalizedDeny, nil
}

func normalizeIPRanges(values []string) ([]string, error) {
	// 统一转换为 CIDR 并排序，使等价输入生成稳定的声明式配置
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if address, err := netip.ParseAddr(value); err == nil {
			value = netip.PrefixFrom(address, address.BitLen()).String()
		} else if prefix, err := netip.ParsePrefix(value); err == nil {
			value = prefix.Masked().String()
		} else {
			return nil, err
		}
		unique[value] = struct{}{}
	}
	return slices.Sorted(maps.Keys(unique)), nil
}
