// Package wasmplugin 处理插件安装、升级和卸载规则。
package wasmplugin

import (
	"context"
	"fmt"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/wasmconfig"
)

// Store 定义插件安装管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page pagination.Request) (pagination.Result[resource.WasmPlugin], error)
	Get(ctx context.Context, pluginID string) (*resource.WasmPlugin, error)
	Create(
		ctx context.Context,
		pluginID string,
		spec resource.WasmPluginSpec,
	) (*resource.WasmPlugin, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.WasmPlugin,
		spec resource.WasmPluginSpec,
	) (*resource.WasmPlugin, error)
	Delete(ctx context.Context, observed *resource.WasmPlugin) error
}

// PolicyUsageLister 定义卸载插件前需要的策略引用查询能力。
type PolicyUsageLister interface {
	ListPolicyUsages(ctx context.Context, packageName string) ([]plugin.PluginPolicyUsage, error)
}

// Catalog 定义插件安装和展示所需的进程内目录能力。
type Catalog interface {
	Items() []CatalogItem
	Lookup(
		sourceID string,
		packageName string,
	) (sourceName string, spec resource.WasmPluginSpec, available bool)
}

// Usecase 协调插件安装、升级和卸载约束。
type Usecase struct {
	store        Store
	policyUsages PolicyUsageLister
	catalog      Catalog
}

// NewUsecase 创建插件安装管理用例。
func NewUsecase(store Store, policyUsages PolicyUsageLister, catalog Catalog) *Usecase {
	return &Usecase{
		store:        store,
		policyUsages: policyUsages,
		catalog:      catalog,
	}
}

// ListCatalog 返回当前所有已启用来源中的可用插件。
func (uc *Usecase) ListCatalog() []CatalogItem {
	return uc.catalog.Items()
}

// CatalogInfo 原子读取已安装插件在当前目录中的来源和升级信息。
func (uc *Usecase) CatalogInfo(plugin *resource.WasmPlugin) CatalogInfo {
	sourceName, spec, available := uc.catalog.Lookup(
		plugin.Spec.SourceID,
		plugin.Spec.Package,
	)
	info := CatalogInfo{SourceName: sourceName}
	if available {
		info.LatestVersion = spec.Version
		info.UpgradeAvailable = newerVersion(plugin.Spec.Version, spec.Version)
	}
	return info
}

// List 返回满足筛选条件的已安装插件。
func (uc *Usecase) List(
	ctx context.Context,
	page pagination.Request,
	filter resourceview.Filter,
) (pagination.Result[resource.WasmPlugin], error) {
	return resourceview.FilterPage(ctx, page, uc.store.ListPage, func(plugin resource.WasmPlugin) bool {
		status := resourceview.WasmPluginStatus(plugin.Generation, plugin.Status.Conditions)
		searchText := strings.Join([]string{
			plugin.Spec.DisplayName,
			plugin.Spec.Package,
			plugin.Spec.Version,
		}, " ")
		return filter.Match(searchText, true, status)
	})
}

// Get 返回指定的已安装插件。
func (uc *Usecase) Get(ctx context.Context, pluginID string) (*resource.WasmPlugin, error) {
	return uc.store.Get(ctx, pluginID)
}

// PolicyUsages 返回仍依赖指定插件包的强类型策略。
func (uc *Usecase) PolicyUsages(
	ctx context.Context,
	packageName string,
) ([]plugin.PluginPolicyUsage, error) {
	return uc.policyUsages.ListPolicyUsages(ctx, packageName)
}

// Install 安装指定来源的目录插件；制品信息由服务端目录决定。
func (uc *Usecase) Install(
	ctx context.Context,
	sourceID string,
	packageName string,
) (*resource.WasmPlugin, error) {
	_, spec, available := uc.catalog.Lookup(sourceID, packageName)
	if !available {
		return nil, adminv1.ErrorBusinessRuleViolation("%s", fmt.Sprintf("插件包 %q 不在选定插件源中", packageName))
	}
	plugin, err := uc.store.Create(ctx, wasmconfig.PluginID(spec.Package), spec)
	if adminv1.IsResourceAlreadyExists(err) {
		return nil, adminv1.ErrorResourceAlreadyExists("%s", fmt.Sprintf("插件包 %q 已安装；如需切换来源，请先卸载现有插件", spec.Package)).WithCause(err)
	}
	return plugin, err
}

// Upgrade 将已安装插件升级到当前目录推荐的版本。
func (uc *Usecase) Upgrade(
	ctx context.Context,
	pluginID string,
	expectedGeneration int64,
) (*resource.WasmPlugin, error) {
	current, err := uc.store.Get(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if current.Generation != expectedGeneration {
		return nil, apperror.ResourceVersionConflict()
	}
	_, spec, available := uc.catalog.Lookup(current.Spec.SourceID, current.Spec.Package)
	if !available {
		return nil, adminv1.ErrorBusinessRuleViolation("%s", fmt.Sprintf("插件包 %q 不在当前插件目录中，无法自动升级", current.Spec.Package))
	}
	if !newerVersion(current.Spec.Version, spec.Version) {
		return nil, adminv1.ErrorBusinessRuleViolation(
			"当前插件已是最新版本",
		)
	}
	return uc.store.ReplaceSpec(ctx, current, spec)
}

// Delete 卸载插件，并在写入前检查当前可见的策略引用。
// 并发写入产生的悬空引用由引用方的 Controller Status 表达。
func (uc *Usecase) Delete(ctx context.Context, pluginID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, pluginID)
	if err != nil {
		return err
	}
	if current.Generation != expectedGeneration {
		return apperror.ResourceVersionConflict()
	}
	if err := uc.checkNotUsed(ctx, current); err != nil {
		return err
	}
	return uc.store.Delete(ctx, current)
}

func (uc *Usecase) checkNotUsed(ctx context.Context, plugin *resource.WasmPlugin) error {
	usages, err := uc.policyUsages.ListPolicyUsages(ctx, plugin.Spec.Package)
	if err != nil {
		return err
	}
	if len(usages) == 0 {
		return nil
	}
	usage := usages[0]
	return adminv1.ErrorResourceReferenced("%s", fmt.Sprintf(
		"插件 %q 仍被%s %q 使用，请先删除策略",
		plugin.Spec.DisplayName,
		usage.PolicyType,
		usage.DisplayName,
	),
	)
}
