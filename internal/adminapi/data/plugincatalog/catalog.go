// Package plugincatalog 加载并聚合官方与自定义插件目录。
package plugincatalog

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/lgc202/ingate/internal/adminapi/biz/pluginsource"
	"github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/httpurl"
)

const (
	officialSourceName  = "Ingate 官方插件源"
	maxCatalogRedirects = 10
)

type sourceDefinition struct {
	id          string
	displayName string
	catalogURL  string
	enabled     bool
	generation  int64
}

type sourceState struct {
	definition  sourceDefinition
	items       []wasmplugin.CatalogItem
	specs       map[string]resource.WasmPluginSpec
	etag        string
	available   bool
	invalidated bool
	observation pluginsource.Observation
}

// Catalog 为并发请求提供按来源隔离的不可变目录视图。
// 每个来源先完整下载和校验，再原子替换自己的视图；单个来源失败不影响其他来源。
type Catalog struct {
	official        sourceDefinition
	store           pluginsource.Store
	refreshInterval time.Duration
	client          *http.Client
	logger          *slog.Logger

	stateMu sync.RWMutex
	states  map[string]sourceState

	sourceSyncMu sync.Mutex
	sourceSync   map[string]chan struct{}

	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
}

// NewCatalog 创建多来源插件目录缓存。
func NewCatalog(
	config *conf.Data,
	store pluginsource.Store,
	logger *slog.Logger,
) *Catalog {
	settings := config.GetPluginCatalog()
	officialURL := settings.GetOfficialSourceUrl()
	return &Catalog{
		official: sourceDefinition{
			id:          pluginsource.OfficialSourceID,
			displayName: officialSourceName,
			catalogURL:  officialURL,
			enabled:     officialURL != "",
		},
		store:           store,
		refreshInterval: settings.GetRefreshInterval().AsDuration(),
		client: &http.Client{
			Timeout: settings.GetTimeout().AsDuration(),
			CheckRedirect: func(request *http.Request, previous []*http.Request) error {
				if len(previous) >= maxCatalogRedirects {
					return fmt.Errorf(
						"%w: plugin catalog redirect limit exceeded",
						pluginsource.ErrSyncFailed,
					)
				}
				if !httpurl.IsValid(request.URL.String()) {
					return fmt.Errorf(
						"%w: plugin catalog redirected to an invalid URL",
						pluginsource.ErrSyncFailed,
					)
				}
				return nil
			},
		},
		logger:     logger,
		states:     make(map[string]sourceState),
		sourceSync: make(map[string]chan struct{}),
	}
}

// Items 返回全部已启用来源的目录项。
func (c *Catalog) Items() []wasmplugin.CatalogItem {
	c.stateMu.RLock()
	items := make([]wasmplugin.CatalogItem, 0)
	for _, state := range c.states {
		if state.available {
			items = append(items, state.items...)
		}
	}
	c.stateMu.RUnlock()

	slices.SortFunc(items, func(left, right wasmplugin.CatalogItem) int {
		return cmp.Or(
			cmp.Compare(left.SourceName, right.SourceName),
			cmp.Compare(left.Name, right.Name),
			cmp.Compare(left.Package, right.Package),
			cmp.Compare(left.SourceID, right.SourceID),
		)
	})
	return items
}

// Lookup 原子读取插件来源名称和当前可安装配置。
// 来源停用时仍返回来源名称，但 available 为 false。
func (c *Catalog) Lookup(
	sourceID string,
	packageName string,
) (sourceName string, spec resource.WasmPluginSpec, available bool) {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	state, exists := c.states[sourceID]
	if !exists {
		return "", resource.WasmPluginSpec{}, false
	}
	if !state.available {
		return state.definition.displayName, resource.WasmPluginSpec{}, false
	}
	spec, available = state.specs[packageName]
	return state.definition.displayName, spec, available
}

// OfficialSource 返回进程配置中的官方插件源。
func (c *Catalog) OfficialSource() pluginsource.Source {
	if c.official.catalogURL == "" {
		return pluginsource.Source{}
	}
	return pluginsource.Source{
		ID:          c.official.id,
		DisplayName: c.official.displayName,
		URL:         c.official.catalogURL,
		Builtin:     true,
		Enabled:     true,
		Observation: c.Observation(c.official.id),
	}
}

// Observation 返回一个来源最近一次同步的进程内观测。
func (c *Catalog) Observation(sourceID string) pluginsource.Observation {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	state, exists := c.states[sourceID]
	if !exists {
		return pluginsource.Observation{State: pluginsource.SyncStateNotSynced}
	}
	return state.observation
}

// InvalidateSource 在声明式配置改变后立即停止从旧目录安装插件。
func (c *Catalog) InvalidateSource(source *resource.PluginSource) {
	definition := definitionFromResource(source)
	state := c.loadSourceState(definition.id)
	applySourceDefinition(&state, definition)
	state.available = false
	if definition.enabled {
		state.observation = pluginsource.Observation{State: pluginsource.SyncStateNotSynced}
	} else {
		state.observation = pluginsource.Observation{State: pluginsource.SyncStateDisabled}
	}
	c.storeSourceState(state)
}

// SyncSource 立即同步一个官方或自定义插件源。
func (c *Catalog) SyncSource(ctx context.Context, sourceID string) error {
	definition, err := c.sourceDefinition(ctx, sourceID)
	if err != nil {
		return err
	}
	return c.syncSource(ctx, definition)
}

// ForgetSource 立即停用已删除来源，并留下阻止旧同步结果提交的版本标记。
func (c *Catalog) ForgetSource(source *resource.PluginSource) {
	if source.Name == pluginsource.OfficialSourceID {
		return
	}
	c.storeSourceState(sourceState{
		definition:  definitionFromResource(source),
		invalidated: true,
		observation: pluginsource.Observation{State: pluginsource.SyncStateNotSynced},
	})
}

// Start 启动全部插件源的后台同步；远程来源不可用不会阻塞 Admin API 启动。
func (c *Catalog) Start(ctx context.Context) {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.cancel != nil {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	c.cancel = cancel
	c.done = done
	go c.run(runCtx, done)
}

// Stop 停止周期同步并等待后台任务退出。
func (c *Catalog) Stop(ctx context.Context) error {
	c.lifecycleMu.Lock()
	cancel := c.cancel
	done := c.done
	c.lifecycleMu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Catalog) finishRun(done chan struct{}) {
	c.lifecycleMu.Lock()
	if c.done == done {
		c.cancel = nil
		c.done = nil
	}
	c.lifecycleMu.Unlock()
	close(done)
}
