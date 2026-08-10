package iprestriction

import (
	"maps"
	"net/netip"
	"slices"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func buildIPRestrictionPolicySpec(
	name string,
	enabled bool,
	targets []*adminv1.PolicyTargetRef,
	allow, deny []string,
) (resource.IPRestrictionPolicySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.IPRestrictionPolicySpec{}, adminservice.BadRequest("IP 访问限制策略名称不能为空")
	}
	refs, err := adminservice.BuildPolicyTargetRefs(targets)
	if err != nil {
		return resource.IPRestrictionPolicySpec{}, err
	}
	if (len(allow) > 0) == (len(deny) > 0) {
		return resource.IPRestrictionPolicySpec{}, adminservice.BadRequest("IP 允许列表和拒绝列表必须且只能配置一个")
	}

	allow, err = normalizeIPRanges(allow)
	if err != nil {
		return resource.IPRestrictionPolicySpec{}, adminservice.BadRequest("IP 允许列表包含无效的 IP 地址或 CIDR")
	}
	deny, err = normalizeIPRanges(deny)
	if err != nil {
		return resource.IPRestrictionPolicySpec{}, adminservice.BadRequest("IP 拒绝列表包含无效的 IP 地址或 CIDR")
	}
	return resource.IPRestrictionPolicySpec{
		DisplayName: name,
		Enabled:     enabled,
		TargetRefs:  refs,
		Allow:       allow,
		Deny:        deny,
	}, nil
}

func normalizeIPRanges(values []string) ([]string, error) {
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
