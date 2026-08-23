// Package pluginsource 处理插件目录来源及其同步生命周期
package pluginsource

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

const OfficialSourceID = "official"

// SyncState 表示最近一次目录同步结果
type SyncState string

const (
	SyncStateReady     SyncState = "Ready"
	SyncStateError     SyncState = "Error"
	SyncStateDisabled  SyncState = "Disabled"
	SyncStateNotSynced SyncState = "NotSynced"
)

// Observation 是远程目录的进程内观测结果，不作为声明式事实持久化
type Observation struct {
	State        SyncState
	Message      string
	PluginCount  int
	LastSyncedAt time.Time
}

// Source 汇总持久化配置与当前进程的同步观测
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

// ListFilter 表达插件源列表筛选条件
type ListFilter struct {
	Query   string
	Enabled *bool
	State   SyncState
}

// Repository 定义自定义插件源需要的持久化能力
type Repository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.PluginSource], error)
	Get(ctx context.Context, sourceID string) (*resource.PluginSource, error)
	Create(ctx context.Context, sourceID string, spec resource.PluginSourceSpec) (*resource.PluginSource, error)
	Update(ctx context.Context, sourceID string, generation int64, spec resource.PluginSourceSpec) (*resource.PluginSource, error)
	Delete(ctx context.Context, sourceID string, generation int64) error
}

// Catalog 提供插件源同步及其进程内观测结果
type Catalog interface {
	OfficialSource() Source
	Observation(sourceID string) Observation
	SyncSource(ctx context.Context, sourceID string) error
	ForgetSource(sourceID string)
}

// Service 协调插件源唯一性、持久化和目录同步
type Service struct {
	repository Repository
	catalog    Catalog
}

// NewService 创建插件源业务服务
func NewService(repository Repository, catalog Catalog) *Service {
	return &Service{repository: repository, catalog: catalog}
}

// List 返回官方插件源和全部自定义插件源
func (s *Service) List(ctx context.Context, filter ListFilter) ([]Source, error) {
	sources := make([]Source, 0)
	official := s.catalog.OfficialSource()
	if official.URL != "" {
		sources = append(sources, official)
	}
	err := biz.VisitPages(ctx, s.repository.ListPage, func(source resource.PluginSource) (bool, error) {
		sources = append(sources, s.source(&source))
		return false, nil
	})
	slices.SortStableFunc(sources, func(left, right Source) int {
		if left.Builtin != right.Builtin {
			if left.Builtin {
				return -1
			}
			return 1
		}
		return cmp.Compare(left.DisplayName, right.DisplayName)
	})
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	return slices.DeleteFunc(sources, func(source Source) bool {
		if query != "" && !strings.Contains(strings.ToLower(source.DisplayName+" "+source.URL), query) {
			return true
		}
		if filter.Enabled != nil && source.Enabled != *filter.Enabled {
			return true
		}
		return filter.State != "" && source.Observation.State != filter.State
	}), err
}

// Get 返回一个官方或自定义插件源
func (s *Service) Get(ctx context.Context, sourceID string) (Source, error) {
	if sourceID == OfficialSourceID {
		source := s.catalog.OfficialSource()
		if source.URL == "" {
			return Source{}, biz.ErrResourceNotFound
		}
		return source, nil
	}
	source, err := s.repository.Get(ctx, sourceID)
	if err != nil {
		return Source{}, err
	}
	return s.source(source), nil
}

// Create 保存自定义插件源并立即尝试首次同步
// 远程目录暂时不可用不会回滚配置，用户可以修正地址或稍后重试
func (s *Service) Create(ctx context.Context, spec resource.PluginSourceSpec) (Source, error) {
	if err := s.ensureIdentityAvailable(ctx, "", spec); err != nil {
		return Source{}, err
	}
	created, err := s.repository.Create(ctx, uuid.NewString(), spec)
	if err != nil {
		return Source{}, err
	}
	_ = s.catalog.SyncSource(ctx, created.Name)
	return s.source(created), nil
}

// Update 修改自定义插件源并重新同步目录
func (s *Service) Update(
	ctx context.Context,
	sourceID string,
	version int64,
	spec resource.PluginSourceSpec,
) (Source, error) {
	current, err := s.repository.Get(ctx, sourceID)
	if err != nil {
		return Source{}, err
	}
	if version != current.Generation {
		return Source{}, sourceVersionConflict(current)
	}
	if err := s.ensureIdentityAvailable(ctx, sourceID, spec); err != nil {
		return Source{}, err
	}
	updated, err := s.repository.Update(ctx, sourceID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return Source{}, sourceVersionConflict(current)
		}
		return Source{}, err
	}
	// 停用也通过目录同步入口更新进程内状态，保留来源名称供已安装插件展示
	// 地址变化由 Catalog 自行丢弃旧目录；仅修改名称时仍可保留最后一次成功结果
	_ = s.catalog.SyncSource(ctx, sourceID)
	return s.source(updated), nil
}

// Delete 删除自定义插件源；已安装插件仍保留当前制品配置
func (s *Service) Delete(ctx context.Context, sourceID string, version int64) error {
	current, err := s.repository.Get(ctx, sourceID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return sourceVersionConflict(current)
	}
	if err := s.repository.Delete(ctx, sourceID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return sourceVersionConflict(current)
		}
		return err
	}
	s.catalog.ForgetSource(sourceID)
	return nil
}

// Sync 立即刷新一个已启用的插件源
func (s *Service) Sync(ctx context.Context, sourceID string) (Source, error) {
	source, err := s.Get(ctx, sourceID)
	if err != nil {
		return Source{}, err
	}
	if !source.Enabled {
		return Source{}, biz.NewRuleViolation("请先启用插件源再同步")
	}
	if err := s.catalog.SyncSource(ctx, sourceID); err != nil {
		return Source{}, biz.NewRuleViolation("插件源同步失败，请检查目录地址和内容")
	}
	return s.Get(ctx, sourceID)
}

func (s *Service) source(source *resource.PluginSource) Source {
	observation := s.catalog.Observation(source.Name)
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

func sourceUpdatedAt(source *resource.PluginSource) time.Time {
	updatedAt, err := time.Parse(time.RFC3339Nano, source.Annotations[resource.AnnotationUpdatedAt])
	if err != nil {
		return source.CreationTimestamp.Time
	}
	return updatedAt
}

func (s *Service) ensureIdentityAvailable(ctx context.Context, sourceID string, spec resource.PluginSourceSpec) error {
	official := s.catalog.OfficialSource()
	if official.URL != "" && (official.DisplayName == spec.DisplayName || official.URL == spec.URL) {
		return biz.NewRuleViolation("插件源名称或目录地址已存在")
	}
	return biz.VisitPages(ctx, s.repository.ListPage, func(source resource.PluginSource) (bool, error) {
		if source.Name == sourceID {
			return false, nil
		}
		if source.Spec.DisplayName == spec.DisplayName || source.Spec.URL == spec.URL {
			return true, biz.NewRuleViolation("插件源名称或目录地址已存在")
		}
		return false, nil
	})
}

func sourceVersionConflict(source *resource.PluginSource) error {
	return biz.NewVersionConflict(
		source.Name,
		fmt.Sprintf("插件源 %q 已被其他用户修改，请刷新后重试", source.Spec.DisplayName),
	)
}
