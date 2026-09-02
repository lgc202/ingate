package pluginsource

import (
	stderrors "errors"
	"time"

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

func sourceUpdatedAt(source *resource.PluginSource) time.Time {
	updatedAt, err := time.Parse(time.RFC3339Nano, source.Annotations[resource.AnnotationUpdatedAt])
	if err != nil {
		return source.CreationTimestamp.Time
	}
	return updatedAt
}
