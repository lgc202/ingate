package iprestriction

import (
	"maps"
	"slices"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/target"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	apivalidation "github.com/lgc202/ingate/internal/pkg/apis/gateway/validation"
)

func parseSpec(
	displayName string,
	enabled bool,
	targetConfigs []*adminv1.PolicyTargetRef,
	allowRanges []string,
	denyRanges []string,
) (resource.IPRestrictionPolicySpec, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return resource.IPRestrictionPolicySpec{}, adminv1.ErrorInvalidArgument("IP 访问限制策略名称不能为空")
	}
	targets, err := target.Refs(
		targetConfigs,
		resource.KindGateway,
		resource.KindRoute,
	)
	if err != nil {
		return resource.IPRestrictionPolicySpec{}, err
	}
	allow, deny, err := parseRanges(allowRanges, denyRanges)
	if err != nil {
		return resource.IPRestrictionPolicySpec{}, err
	}

	return resource.IPRestrictionPolicySpec{
		DisplayName: displayName,
		Enabled:     enabled,
		TargetRefs:  targets,
		Allow:       allow,
		Deny:        deny,
	}, nil
}

func parseRanges(allow, deny []string) ([]string, []string, error) {
	if (len(allow) > 0) == (len(deny) > 0) {
		return nil, nil, adminv1.ErrorInvalidArgument("IP 允许列表和拒绝列表必须且只能配置一个")
	}

	normalizedAllow, err := parseRangeList(allow, "IP 允许列表")
	if err != nil {
		return nil, nil, err
	}
	normalizedDeny, err := parseRangeList(deny, "IP 拒绝列表")
	if err != nil {
		return nil, nil, err
	}
	return normalizedAllow, normalizedDeny, nil
}

func parseRangeList(values []string, listName string) ([]string, error) {
	if len(values) > apivalidation.MaxRanges {
		return nil, adminv1.ErrorInvalidArgument("%s", listName+"数量超过限制")
	}

	// 统一转换为 CIDR 并排序，使等价输入生成稳定的声明式配置。
	unique := make(map[string]bool, len(values))
	for _, value := range values {
		normalized, valid := apivalidation.NormalizeRange(value)
		if !valid {
			return nil, adminv1.ErrorInvalidArgument("%s", listName+"包含无效的 IP 地址或 CIDR")
		}
		unique[normalized] = true
	}
	return slices.Sorted(maps.Keys(unique)), nil
}
