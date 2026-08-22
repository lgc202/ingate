// Package wasmplugin 处理插件安装、升级和卸载规则
package wasmplugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Repository 定义插件安装管理需要的持久化能力
type Repository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.WasmPlugin], error)
	Get(ctx context.Context, pluginID string) (*resource.WasmPlugin, error)
	Create(ctx context.Context, pluginID string, spec resource.WasmPluginSpec) (*resource.WasmPlugin, error)
	Update(ctx context.Context, pluginID string, generation int64, spec resource.WasmPluginSpec) (*resource.WasmPlugin, error)
	Delete(ctx context.Context, pluginID string, generation int64) error
}

// HeaderTransformationPolicyRepository 定义卸载标准转换插件前的策略引用检查
type HeaderTransformationPolicyRepository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.HeaderTransformationPolicy], error)
}

// Service 协调插件包唯一性、升级和卸载约束
type Service struct {
	repository            Repository
	headerTransformations HeaderTransformationPolicyRepository
	catalog               Catalog
}

// NewService 创建插件安装管理服务
func NewService(
	repository Repository,
	headerTransformations HeaderTransformationPolicyRepository,
	catalog Catalog,
) *Service {
	return &Service{
		repository:            repository,
		headerTransformations: headerTransformations,
		catalog:               catalog,
	}
}

// Catalog 返回当前可用的官方插件目录
func (s *Service) Catalog() CatalogSnapshot {
	return s.catalog.Snapshot()
}

// RefreshCatalog 立即检查远程插件目录
// 拉取失败不会替换内存中的上次成功结果
func (s *Service) RefreshCatalog(ctx context.Context) (CatalogSnapshot, error) {
	if err := s.catalog.Refresh(ctx); err != nil {
		return CatalogSnapshot{}, catalogUnavailable(err)
	}
	return s.catalog.Snapshot(), nil
}

// UpgradeVersion 返回插件当前是否存在可用的新版本
// 版本比较由后端统一完成，控制台不需要理解语义版本规则
func (s *Service) UpgradeVersion(packageName, currentVersion string) (string, bool) {
	spec, ok := s.catalog.PluginSpec(packageName)
	if !ok {
		return "", false
	}
	return spec.Version, newerVersion(currentVersion, spec.Version)
}

// List 查询已安装插件
func (s *Service) List(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.WasmPlugin], error) {
	return s.repository.ListPage(ctx, page)
}

// Get 查询单个已安装插件
func (s *Service) Get(ctx context.Context, pluginID string) (*resource.WasmPlugin, error) {
	return s.repository.Get(ctx, pluginID)
}

// Install 安装目录中的官方插件；制品信息由服务端目录决定
func (s *Service) Install(ctx context.Context, packageName string) (*resource.WasmPlugin, error) {
	spec, ok := s.catalog.PluginSpec(packageName)
	if !ok {
		return nil, biz.NewRuleViolation(fmt.Sprintf("插件包 %q 不在当前插件目录中", packageName))
	}
	if err := s.ensureIdentityAvailable(ctx, "", spec.DisplayName, spec.Package); err != nil {
		return nil, err
	}
	return s.repository.Create(ctx, uuid.NewString(), spec)
}

// Upgrade 将已安装插件升级到当前目录推荐的版本
func (s *Service) Upgrade(
	ctx context.Context,
	pluginID string,
	version int64,
) (*resource.WasmPlugin, error) {
	current, err := s.repository.Get(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, versionConflict(current)
	}
	spec, ok := s.catalog.PluginSpec(current.Spec.Package)
	if !ok {
		return nil, biz.NewRuleViolation(fmt.Sprintf("插件包 %q 不在当前插件目录中，无法自动升级", current.Spec.Package))
	}
	updated, err := s.repository.Update(ctx, pluginID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return nil, versionConflict(current)
		}
		return nil, err
	}
	return updated, nil
}

// Delete 卸载插件；仍有策略依赖时必须先删除策略
func (s *Service) Delete(ctx context.Context, pluginID string, version int64) error {
	current, err := s.repository.Get(ctx, pluginID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return versionConflict(current)
	}
	if err := s.ensureNotUsed(ctx, current); err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, pluginID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return versionConflict(current)
		}
		return err
	}
	return nil
}

func (s *Service) ensureIdentityAvailable(ctx context.Context, pluginID, displayName, packageName string) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(plugin resource.WasmPlugin) (bool, error) {
		if plugin.Name == pluginID {
			return false, nil
		}
		if plugin.Spec.DisplayName == displayName {
			return true, biz.NewRuleViolation(fmt.Sprintf("插件名称 %q 已存在", displayName))
		}
		if plugin.Spec.Package == packageName {
			return true, biz.NewRuleViolation(fmt.Sprintf("插件包 %q 已安装，请直接升级现有插件", packageName))
		}
		return false, nil
	})
}

func (s *Service) ensureNotUsed(ctx context.Context, plugin *resource.WasmPlugin) error {
	if plugin.Spec.Package != resource.WasmPluginPackageTransformer {
		return nil
	}
	return biz.VisitPages(ctx, s.headerTransformations.ListPage, func(policy resource.HeaderTransformationPolicy) (bool, error) {
		return true, biz.NewRuleViolation(fmt.Sprintf(
			"插件 %q 仍被请求响应转换策略 %q 使用，请先删除策略",
			plugin.Spec.DisplayName,
			policy.Spec.DisplayName,
		))
	})
}

func versionConflict(plugin *resource.WasmPlugin) error {
	return biz.NewVersionConflict(
		plugin.Name,
		fmt.Sprintf("插件 %q 已被其他用户修改，请刷新后重试", plugin.Spec.DisplayName),
	)
}
