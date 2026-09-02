// Package pluginsource 处理插件目录来源及其同步生命周期。
package pluginsource

import (
	"context"
	stderrors "errors"

	"github.com/google/uuid"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义自定义插件源管理所需的持久化能力。
type Store interface {
	ListPage(ctx context.Context, page pagination.Request) (pagination.Result[resource.PluginSource], error)
	Get(ctx context.Context, sourceID string) (*resource.PluginSource, error)
	Create(
		ctx context.Context,
		sourceID string,
		spec resource.PluginSourceSpec,
	) (*resource.PluginSource, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.PluginSource,
		spec resource.PluginSourceSpec,
	) (*resource.PluginSource, error)
	Delete(ctx context.Context, observed *resource.PluginSource) error
}

// Catalog 提供插件源同步及其进程内观测结果。
type Catalog interface {
	OfficialSource() Source
	Observation(sourceID string) Observation
	InvalidateSource(source *resource.PluginSource)
	SyncSource(ctx context.Context, sourceID string) error
	ForgetSource(source *resource.PluginSource)
}

// Usecase 协调插件源持久化和目录同步。
type Usecase struct {
	store   Store
	catalog Catalog
}

// NewUsecase 创建插件源用例。
func NewUsecase(store Store, catalog Catalog) *Usecase {
	return &Usecase{store: store, catalog: catalog}
}

// Get 返回指定的官方或自定义插件源。
func (uc *Usecase) Get(ctx context.Context, sourceID string) (Source, error) {
	if sourceID == OfficialSourceID {
		source := uc.catalog.OfficialSource()
		if source.URL == "" {
			return Source{}, apperror.ResourceNotFound()
		}
		return source, nil
	}
	source, err := uc.store.Get(ctx, sourceID)
	if err != nil {
		return Source{}, err
	}
	return uc.sourceFromResource(source), nil
}

// Create 保存自定义插件源并立即尝试首次同步。
// 远程目录暂时不可用时仍保留配置，用户可以修正地址或稍后重试。
func (uc *Usecase) Create(ctx context.Context, spec resource.PluginSourceSpec) (Source, error) {
	source, err := uc.store.Create(ctx, uuid.NewString(), spec)
	if err != nil {
		return Source{}, err
	}
	uc.catalog.InvalidateSource(source)
	// 持久化成功不依赖远程目录可用性；同步结果由来源观测状态表达。
	_ = uc.catalog.SyncSource(ctx, source.Name)
	return uc.sourceFromResource(source), nil
}

// Replace 使用配置版本完整替换自定义插件源配置并重新同步。
func (uc *Usecase) Replace(
	ctx context.Context,
	sourceID string,
	expectedGeneration int64,
	spec resource.PluginSourceSpec,
) (Source, error) {
	current, err := uc.store.Get(ctx, sourceID)
	if err != nil {
		return Source{}, err
	}
	if current.Generation != expectedGeneration {
		return Source{}, apperror.ResourceVersionConflict()
	}
	source, err := uc.store.ReplaceSpec(ctx, current, spec)
	if err != nil {
		return Source{}, err
	}
	uc.catalog.InvalidateSource(source)
	// 同步边界也处理停用状态；持久化成功不依赖远程目录可用性。
	_ = uc.catalog.SyncSource(ctx, sourceID)
	return uc.sourceFromResource(source), nil
}

// Delete 删除自定义插件源；已安装插件仍保留当前制品配置。
func (uc *Usecase) Delete(ctx context.Context, sourceID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, sourceID)
	if err != nil {
		return err
	}
	if current.Generation != expectedGeneration {
		return apperror.ResourceVersionConflict()
	}
	if err := uc.store.Delete(ctx, current); err != nil {
		return err
	}
	uc.catalog.ForgetSource(current)
	return nil
}

// Sync 立即刷新一个已启用的插件源。
func (uc *Usecase) Sync(ctx context.Context, sourceID string) (Source, error) {
	source, err := uc.Get(ctx, sourceID)
	if err != nil {
		return Source{}, err
	}
	if !source.Enabled {
		return Source{}, adminv1.ErrorBusinessRuleViolation(
			"请先启用插件源再同步",
		)
	}
	if err := uc.catalog.SyncSource(ctx, sourceID); err != nil {
		if ctx.Err() != nil {
			return Source{}, ctx.Err()
		}
		if adminv1.IsResourceNotFound(err) {
			return Source{}, apperror.ResourceNotFound()
		}
		if stderrors.Is(err, ErrSyncUnavailable) {
			return Source{}, apperror.DependencyUnavailable("插件源暂时不可用，请稍后重试", err)
		}
		if stderrors.Is(err, ErrSyncFailed) {
			return Source{}, adminv1.ErrorBusinessRuleViolation(
				"插件源同步失败，请检查目录地址和内容",
			).WithCause(err)
		}
		return Source{}, err
	}
	return uc.Get(ctx, sourceID)
}
