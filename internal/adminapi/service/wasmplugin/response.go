package wasmplugin

import (
	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	pluginbiz "github.com/lgc202/ingate/internal/adminapi/biz/plugin"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	wasmpluginbiz "github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func catalogResponse(items []wasmpluginbiz.CatalogItem) *adminv1.ListWasmPluginCatalogResponse {
	return &adminv1.ListWasmPluginCatalogResponse{
		Plugins: lo.Map(items, func(item wasmpluginbiz.CatalogItem, _ int) *adminv1.WasmPluginCatalogItem {
			return catalogItemResponse(item)
		}),
	}
}

func catalogItemResponse(item wasmpluginbiz.CatalogItem) *adminv1.WasmPluginCatalogItem {
	return &adminv1.WasmPluginCatalogItem{
		SourceId:      item.SourceID,
		SourceName:    item.SourceName,
		PackageName:   item.Package,
		Name:          item.Name,
		PluginVersion: item.Version,
		Category:      item.Category,
		Description:   item.Description,
		Provider:      item.Provider,
		License:       item.License,
		SourceUrl:     item.SourceURL,
	}
}

func pluginResponse(
	plugin *resource.WasmPlugin,
	catalog wasmpluginbiz.CatalogInfo,
	usages []pluginbiz.PolicyUsage,
) *adminv1.WasmPlugin {
	status := resourceview.WasmPluginStatus(plugin.Generation, plugin.Status.Conditions)
	response := &adminv1.WasmPlugin{
		Id:               plugin.Name,
		SourceId:         plugin.Spec.SourceID,
		SourceName:       catalog.SourceName,
		Name:             plugin.Spec.DisplayName,
		PackageName:      plugin.Spec.Package,
		PluginVersion:    plugin.Spec.Version,
		Url:              plugin.Spec.URL,
		Sha256:           plugin.Spec.SHA256,
		PullPolicy:       pullPolicyResponse(plugin.Spec.PullPolicy),
		State:            adminservice.ResourceState(status.State),
		Message:          pluginStatusMessage(status),
		Version:          plugin.Generation,
		CreatedAt:        adminservice.Timestamp(plugin.CreationTimestamp.Time),
		UpdatedAt:        adminservice.Timestamp(adminservice.ResourceUpdatedAt(plugin.Annotations)),
		LatestVersion:    catalog.LatestVersion,
		UpgradeAvailable: catalog.UpgradeAvailable,
		Usages: lo.Map(usages, func(usage pluginbiz.PolicyUsage, _ int) *adminv1.WasmPluginPolicyUsage {
			return &adminv1.WasmPluginPolicyUsage{
				PolicyId:   usage.PolicyID,
				PolicyKind: string(usage.PolicyKind),
				PolicyName: usage.DisplayName,
			}
		}),
	}
	return response
}

func pluginStatusMessage(status resourceview.Status) string {
	switch status.State {
	case resourceview.StateReady:
		return "插件已就绪"
	case resourceview.StatePending:
		return "正在准备插件"
	case resourceview.StateError:
		if status.Reason == resourceview.ReasonArtifactUnavailable {
			return "插件制品暂时无法加载，系统将自动重试；请确认插件已发布且当前环境有权访问"
		}
		return "插件不可用，请检查制品地址和摘要"
	default:
		return ""
	}
}

func pullPolicyResponse(value resource.WasmPluginPullPolicy) adminv1.WasmPluginPullPolicy {
	if value == resource.WasmPluginPullAlways {
		return adminv1.WasmPluginPullPolicy_WASM_PLUGIN_PULL_POLICY_ALWAYS
	}
	return adminv1.WasmPluginPullPolicy_WASM_PLUGIN_PULL_POLICY_IF_NOT_PRESENT
}
