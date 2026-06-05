package dto

import (
	runtimegroupsvc "github.com/lgc202/ingate/internal/adminapi/service/runtimegroup"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewListRuntimeGroupsResp 转换 RuntimeGroup 列表用例结果为 HTTP 响应
func NewListRuntimeGroupsResp(result *runtimegroupsvc.ListResult) ListRuntimeGroupsResp {
	runtimeGroups := make([]RuntimeGroupSummary, 0, len(result.RuntimeGroups))
	for i := range result.RuntimeGroups {
		runtimeGroups = append(runtimeGroups, runtimeGroupSummary(&result.RuntimeGroups[i]))
	}
	return ListRuntimeGroupsResp{RuntimeGroups: runtimeGroups}
}

func runtimeGroupSummary(runtimeGroup *resource.RuntimeGroup) RuntimeGroupSummary {
	return RuntimeGroupSummary{
		ID:          runtimeGroup.Name,
		DisplayName: displayNameOrID(runtimeGroup.Name, runtimeGroup.Spec.DisplayName),
		Description: runtimeGroup.Spec.Description,
		Enabled:     runtimeGroup.Spec.Enabled,
		Target:      runtimeGroup.Spec.TargetRef.Name,
	}
}

func displayNameOrID(id, displayName string) string {
	if displayName != "" {
		return displayName
	}
	return id
}
