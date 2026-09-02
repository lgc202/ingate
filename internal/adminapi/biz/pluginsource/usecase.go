// Package pluginsource 处理插件目录来源及其同步生命周期。
package pluginsource

import (
	"cmp"
	"context"
	stderrors "errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// OfficialSourceID 是进程配置中官方插件源的稳定资源标识。
const OfficialSourceID = "00000000-0000-5000-8000-000000000001"

// ErrSyncFailed 表示远程响应不符合插件目录协议。
var ErrSyncFailed = stderrors.New("plugin source sync failed")

// ErrSyncUnavailable 表示远程插件目录当前无法访问。
var ErrSyncUnavailable = stderrors.New("plugin source is unavailable")

// SyncState 表示最近一次目录同步结果。
type SyncState string

const (
	// SyncStateReady 表示最近一次同步成功。
	SyncStateReady SyncState = "Ready"
	// SyncStateError 表示最近一次同步失败。
	SyncStateError SyncState = "Error"
	// SyncStateDisabled 表示来源已停用。
	SyncStateDisabled SyncState = "Disabled"
	// SyncStateNotSynced 表示尚未完成首次同步。
	SyncStateNotSynced SyncState = "NotSynced"
)

// Observation 是远程目录的进程内观测结果，不作为声明式事实持久化。
type Observation struct {
	State        SyncState
	Message      string
	PluginCount  int
	LastSyncedAt time.Time
}

// Source 汇总持久化配置与当前进程的同步观测。
type Source struct {
	ID          string
	DisplayName string
	URL         string
	Builtin     bool
	Enabled     bool
	Observation Observation
	Generation  int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListFilter 表达插件源列表筛选条件。
type ListFilter struct {
	Query   string
	Enabled *bool
	State   SyncState
}

// SourcePage 保存一页插件源及下一页游标。
type SourcePage struct {
	Items      []Source
	NextCursor string
}

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

// List 返回满足筛选条件的官方与自定义插件源。
func (uc *Usecase) List(
	ctx context.Context,
	page pagination.Request,
	filter ListFilter,
) (SourcePage, error) {
	filter.Query = strings.ToLower(strings.TrimSpace(filter.Query))
	sources := make([]Source, 0)
	if official := uc.catalog.OfficialSource(); official.URL != "" {
		sources = append(sources, official)
	}
	if err := pagination.VisitPages(ctx, uc.store.ListPage, func(source resource.PluginSource) (bool, error) {
		sources = append(sources, uc.sourceFromResource(&source))
		return false, nil
	}); err != nil {
		return SourcePage{}, err
	}

	slices.SortStableFunc(sources, compareSources)
	sources = slices.DeleteFunc(sources, func(source Source) bool {
		return !filter.matches(source)
	})
	start, err := sourcePageStart(sources, page.Cursor)
	if err != nil {
		return SourcePage{}, err
	}
	end := min(start+int(page.Limit), len(sources))
	items := slices.Clone(sources[start:end])

	var nextCursor string
	if end < len(sources) && len(items) > 0 {
		nextCursor = items[len(items)-1].ID
	}
	return SourcePage{Items: items, NextCursor: nextCursor}, nil
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

func (uc *Usecase) sourceFromResource(source *resource.PluginSource) Source {
	observation := uc.catalog.Observation(source.Name)
	if !source.Spec.Enabled {
		observation = Observation{State: SyncStateDisabled}
	}
	return Source{
		ID:          source.Name,
		DisplayName: source.Spec.DisplayName,
		URL:         source.Spec.URL,
		Enabled:     source.Spec.Enabled,
		Observation: observation,
		Generation:  source.Generation,
		CreatedAt:   source.CreationTimestamp.Time,
		UpdatedAt:   sourceUpdatedAt(source),
	}
}

func (f ListFilter) matches(source Source) bool {
	if f.Query != "" && !strings.Contains(
		strings.ToLower(source.DisplayName+" "+source.URL),
		f.Query,
	) {
		return false
	}
	if f.Enabled != nil && source.Enabled != *f.Enabled {
		return false
	}
	return f.State == "" || source.Observation.State == f.State
}

func compareSources(left, right Source) int {
	if left.Builtin != right.Builtin {
		if left.Builtin {
			return -1
		}
		return 1
	}
	if result := cmp.Compare(left.DisplayName, right.DisplayName); result != 0 {
		return result
	}
	return cmp.Compare(left.ID, right.ID)
}

func sourcePageStart(sources []Source, cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	for i := range sources {
		if sources[i].ID == cursor {
			return i + 1, nil
		}
	}
	return 0, apperror.InvalidCursor(nil)
}

func sourceUpdatedAt(source *resource.PluginSource) time.Time {
	updatedAt, err := time.Parse(time.RFC3339Nano, source.Annotations[resource.AnnotationUpdatedAt])
	if err != nil {
		return source.CreationTimestamp.Time
	}
	return updatedAt
}
