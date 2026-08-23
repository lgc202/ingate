// Package plugincatalog 加载并聚合官方与自定义插件目录
package plugincatalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/biz/pluginsource"
	"github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const (
	maxCatalogBytes    = 1 << 20
	officialSourceName = "Ingate 官方插件源"
)

type sourceDefinition struct {
	id      string
	name    string
	url     string
	enabled bool
}

type sourceState struct {
	definition  sourceDefinition
	items       []wasmplugin.CatalogItem
	specs       map[string]resource.WasmPluginSpec
	etag        string
	observation pluginsource.Observation
}

type catalogFile struct {
	Plugins []catalogPlugin `json:"plugins"`
}

type catalogPlugin struct {
	Package     string           `json:"package"`
	Name        string           `json:"name"`
	Category    string           `json:"category"`
	Description string           `json:"description"`
	Provider    string           `json:"provider"`
	License     string           `json:"license"`
	SourceURL   string           `json:"sourceUrl"`
	Releases    []catalogRelease `json:"releases"`
}

type catalogRelease struct {
	Version          string          `json:"version"`
	MinIngateVersion string          `json:"minIngateVersion"`
	Artifact         catalogArtifact `json:"artifact"`
}

type catalogArtifact struct {
	Repository string `json:"repository"`
	SHA256     string `json:"sha256"`
}

// Catalog 为并发请求提供按来源隔离的不可变目录视图
// 每个来源先完整下载和校验，再原子替换自己的视图；单个来源失败不会影响其他来源
type Catalog struct {
	official        sourceDefinition
	repository      pluginsource.Repository
	refreshInterval time.Duration
	client          *http.Client
	logger          *slog.Logger

	syncMu  sync.Mutex
	stateMu sync.RWMutex
	states  map[string]sourceState

	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
}

// NewCatalog 创建多来源插件目录缓存
func NewCatalog(config *conf.Data, repository pluginsource.Repository, logger *slog.Logger) (*Catalog, error) {
	settings := config.GetPluginCatalog()
	officialURL := strings.TrimSpace(settings.GetOfficialSourceUrl())
	if err := validateCatalogURL(officialURL); officialURL != "" && err != nil {
		return nil, fmt.Errorf("parse official plugin source URL %q: %w", officialURL, err)
	}
	return &Catalog{
		official: sourceDefinition{
			id:      pluginsource.OfficialSourceID,
			name:    officialSourceName,
			url:     officialURL,
			enabled: officialURL != "",
		},
		repository:      repository,
		refreshInterval: settings.GetRefreshInterval().AsDuration(),
		client:          &http.Client{Timeout: settings.GetTimeout().AsDuration()},
		logger:          logger,
		states:          make(map[string]sourceState),
	}, nil
}

// Snapshot 返回全部已启用来源的目录视图
func (c *Catalog) Snapshot() wasmplugin.CatalogSnapshot {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()

	items := make([]wasmplugin.CatalogItem, 0)
	for _, state := range c.states {
		if state.definition.enabled {
			items = append(items, state.items...)
		}
	}
	slices.SortFunc(items, func(left, right wasmplugin.CatalogItem) int {
		if result := strings.Compare(left.SourceName, right.SourceName); result != 0 {
			return result
		}
		if result := strings.Compare(left.Name, right.Name); result != 0 {
			return result
		}
		return strings.Compare(left.Package, right.Package)
	})
	return wasmplugin.CatalogSnapshot{Items: items}
}

// SourceName 返回目录中仍存在的插件源名称
// 来源停用时仍保留其身份，只有删除来源后才返回不存在
func (c *Catalog) SourceName(sourceID string) (string, bool) {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	state, ok := c.states[sourceID]
	if !ok {
		return "", false
	}
	return state.definition.name, true
}

// CatalogItem 返回指定来源中的插件目录项
func (c *Catalog) CatalogItem(sourceID, packageName string) (wasmplugin.CatalogItem, bool) {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	state, ok := c.states[sourceID]
	if !ok || !state.definition.enabled {
		return wasmplugin.CatalogItem{}, false
	}
	for _, item := range state.items {
		if item.Package == packageName {
			return item, true
		}
	}
	return wasmplugin.CatalogItem{}, false
}

// PluginSpec 返回指定来源中插件的最新兼容安装参数
func (c *Catalog) PluginSpec(sourceID, packageName string) (resource.WasmPluginSpec, bool) {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	state, ok := c.states[sourceID]
	if !ok || !state.definition.enabled {
		return resource.WasmPluginSpec{}, false
	}
	spec, ok := state.specs[packageName]
	return spec, ok
}

// OfficialSource 返回进程配置中的官方插件源
func (c *Catalog) OfficialSource() pluginsource.Source {
	if c.official.url == "" {
		return pluginsource.Source{}
	}
	return pluginsource.Source{
		ID:          c.official.id,
		DisplayName: c.official.name,
		URL:         c.official.url,
		Builtin:     true,
		Enabled:     true,
		Observation: c.Observation(c.official.id),
	}
}

// Observation 返回一个来源最近一次同步的进程内观测
func (c *Catalog) Observation(sourceID string) pluginsource.Observation {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	state, ok := c.states[sourceID]
	if !ok {
		return pluginsource.Observation{State: pluginsource.SyncStateNotSynced}
	}
	return state.observation
}

// SyncSource 立即同步一个官方或自定义插件源
func (c *Catalog) SyncSource(ctx context.Context, sourceID string) error {
	definition, err := c.sourceDefinition(ctx, sourceID)
	if err != nil {
		return err
	}
	if !definition.enabled {
		c.setDisabled(definition)
		return nil
	}
	return c.syncSource(ctx, definition)
}

// ForgetSource 从进程缓存移除一个自定义来源
func (c *Catalog) ForgetSource(sourceID string) {
	if sourceID == pluginsource.OfficialSourceID {
		return
	}
	c.stateMu.Lock()
	delete(c.states, sourceID)
	c.stateMu.Unlock()
}

// Start 启动全部插件源的周期同步；生命周期由 Admin API 的 Kratos App 管理
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
	c.syncAllAndLog(runCtx)
	go c.run(runCtx, done)
}

// Stop 停止周期同步并等待后台任务退出
func (c *Catalog) Stop(ctx context.Context) error {
	c.lifecycleMu.Lock()
	cancel := c.cancel
	done := c.done
	c.cancel = nil
	c.done = nil
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

func (c *Catalog) run(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.syncAllAndLog(ctx)
		}
	}
}

func (c *Catalog) syncAllAndLog(ctx context.Context) {
	if err := c.syncAll(ctx); err != nil && ctx.Err() == nil {
		c.logger.Warn("sync plugin sources failed", "error", err)
	}
}

func (c *Catalog) syncAll(ctx context.Context) error {
	definitions := make([]sourceDefinition, 0)
	if c.official.enabled {
		definitions = append(definitions, c.official)
	}
	err := biz.VisitPages(ctx, c.repository.ListPage, func(source resource.PluginSource) (bool, error) {
		definitions = append(definitions, definitionFromResource(&source))
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("list custom plugin sources: %w", err)
	}

	active := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		active[definition.id] = struct{}{}
		if !definition.enabled {
			c.setDisabled(definition)
			continue
		}
		if err := c.syncSource(ctx, definition); err != nil && ctx.Err() == nil {
			c.logger.Warn("sync plugin source failed", "source_id", definition.id, "error", err)
		}
	}
	c.removeDeletedSources(active)
	return nil
}

func (c *Catalog) sourceDefinition(ctx context.Context, sourceID string) (sourceDefinition, error) {
	if sourceID == pluginsource.OfficialSourceID {
		if c.official.url == "" {
			return sourceDefinition{}, biz.ErrResourceNotFound
		}
		return c.official, nil
	}
	source, err := c.repository.Get(ctx, sourceID)
	if err != nil {
		return sourceDefinition{}, err
	}
	return definitionFromResource(source), nil
}

func definitionFromResource(source *resource.PluginSource) sourceDefinition {
	return sourceDefinition{
		id:      source.Name,
		name:    source.Spec.DisplayName,
		url:     source.Spec.URL,
		enabled: source.Spec.Enabled,
	}
}

// syncSource 串行化网络刷新，避免周期任务和手动同步同时覆盖同一来源
func (c *Catalog) syncSource(ctx context.Context, definition sourceDefinition) error {
	c.syncMu.Lock()
	defer c.syncMu.Unlock()

	previous := c.sourceState(definition.id)
	if previous.definition.url != definition.url {
		previous = sourceState{}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, definition.url, nil)
	if err != nil {
		c.recordFailure(definition, previous, err)
		return fmt.Errorf("create plugin source request: %w", err)
	}
	if previous.etag != "" {
		request.Header.Set("If-None-Match", previous.etag)
	}
	response, err := c.client.Do(request)
	if err != nil {
		c.recordFailure(definition, previous, err)
		return fmt.Errorf("fetch plugin source: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusNotModified {
		previous.definition = definition
		previous.observation = pluginsource.Observation{
			State:        pluginsource.SyncStateReady,
			PluginCount:  len(previous.items),
			LastSyncedAt: time.Now(),
		}
		c.storeSource(definition.id, previous)
		return nil
	}
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("unexpected HTTP status %s", response.Status)
		c.recordFailure(definition, previous, err)
		return fmt.Errorf("fetch plugin source: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxCatalogBytes+1))
	if err != nil {
		c.recordFailure(definition, previous, err)
		return fmt.Errorf("read plugin source: %w", err)
	}
	if len(data) > maxCatalogBytes {
		err = fmt.Errorf("response exceeds %d bytes", maxCatalogBytes)
		c.recordFailure(definition, previous, err)
		return fmt.Errorf("read plugin source: %w", err)
	}
	state, err := decodeCatalog(data, definition, version.String())
	if err != nil {
		c.recordFailure(definition, previous, err)
		return fmt.Errorf("decode plugin source: %w", err)
	}
	state.etag = response.Header.Get("ETag")
	state.observation = pluginsource.Observation{
		State:        pluginsource.SyncStateReady,
		PluginCount:  len(state.items),
		LastSyncedAt: time.Now(),
	}
	c.storeSource(definition.id, state)
	return nil
}

func (c *Catalog) sourceState(sourceID string) sourceState {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.states[sourceID]
}

func (c *Catalog) storeSource(sourceID string, state sourceState) {
	c.stateMu.Lock()
	c.states[sourceID] = state
	c.stateMu.Unlock()
}

func (c *Catalog) recordFailure(definition sourceDefinition, previous sourceState, err error) {
	lastSyncedAt := previous.observation.LastSyncedAt
	previous.definition = definition
	previous.observation = pluginsource.Observation{
		State:        pluginsource.SyncStateError,
		Message:      "目录同步失败，请检查地址和目录内容",
		PluginCount:  len(previous.items),
		LastSyncedAt: lastSyncedAt,
	}
	c.storeSource(definition.id, previous)
	c.logger.Warn("plugin source unavailable", "source_id", definition.id, "error", err)
}

func (c *Catalog) setDisabled(definition sourceDefinition) {
	c.storeSource(definition.id, sourceState{
		definition:  definition,
		items:       make([]wasmplugin.CatalogItem, 0),
		specs:       make(map[string]resource.WasmPluginSpec),
		observation: pluginsource.Observation{State: pluginsource.SyncStateDisabled},
	})
}

func (c *Catalog) removeDeletedSources(active map[string]struct{}) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	for sourceID := range c.states {
		if _, ok := active[sourceID]; !ok {
			delete(c.states, sourceID)
		}
	}
}

func decodeCatalog(data []byte, definition sourceDefinition, ingateVersion string) (sourceState, error) {
	var file catalogFile
	if err := json.Unmarshal(data, &file); err != nil {
		return sourceState{}, err
	}
	state := sourceState{
		definition: definition,
		items:      make([]wasmplugin.CatalogItem, 0, len(file.Plugins)),
		specs:      make(map[string]resource.WasmPluginSpec, len(file.Plugins)),
	}
	for _, plugin := range file.Plugins {
		if err := addPlugin(&state, plugin, ingateVersion); err != nil {
			return sourceState{}, err
		}
	}
	return state, nil
}

func addPlugin(state *sourceState, plugin catalogPlugin, ingateVersion string) error {
	packageName := strings.TrimSpace(plugin.Package)
	name := strings.TrimSpace(plugin.Name)
	if packageName == "" || name == "" || len(plugin.Releases) == 0 {
		return fmt.Errorf("plugin catalog entry requires package, name and releases")
	}
	if _, exists := state.specs[packageName]; exists {
		return fmt.Errorf("plugin catalog contains duplicate package %q", packageName)
	}
	release, ok, err := latestCompatibleRelease(plugin.Releases, ingateVersion)
	if err != nil {
		return fmt.Errorf("validate plugin %q releases: %w", packageName, err)
	}
	if !ok {
		return nil
	}
	state.items = append(state.items, wasmplugin.CatalogItem{
		SourceID:    state.definition.id,
		SourceName:  state.definition.name,
		Package:     packageName,
		Name:        name,
		Version:     release.Version,
		Category:    strings.TrimSpace(plugin.Category),
		Description: strings.TrimSpace(plugin.Description),
		Provider:    strings.TrimSpace(plugin.Provider),
		License:     strings.TrimSpace(plugin.License),
		SourceURL:   strings.TrimSpace(plugin.SourceURL),
	})
	state.specs[packageName] = resource.WasmPluginSpec{
		SourceID:    state.definition.id,
		DisplayName: name,
		Package:     packageName,
		Version:     release.Version,
		URL:         fmt.Sprintf("oci://%s:v%s", strings.TrimSpace(release.Artifact.Repository), release.Version),
		SHA256:      strings.TrimPrefix(strings.TrimSpace(release.Artifact.SHA256), "sha256:"),
		PullPolicy:  resource.WasmPluginPullIfNotPresent,
	}
	return nil
}

func latestCompatibleRelease(releases []catalogRelease, ingateVersion string) (catalogRelease, bool, error) {
	var selected catalogRelease
	for _, release := range releases {
		release.Version = strings.TrimPrefix(strings.TrimSpace(release.Version), "v")
		release.MinIngateVersion = canonicalVersion(strings.TrimSpace(release.MinIngateVersion))
		release.Artifact.Repository = strings.TrimSpace(release.Artifact.Repository)
		release.Artifact.SHA256 = strings.TrimPrefix(strings.TrimSpace(release.Artifact.SHA256), "sha256:")
		if !semver.IsValid(canonicalVersion(release.Version)) || release.Artifact.Repository == "" {
			return catalogRelease{}, false, fmt.Errorf("release requires a semantic version and artifact repository")
		}
		if !validSHA256(release.Artifact.SHA256) {
			return catalogRelease{}, false, fmt.Errorf("release %q has invalid SHA-256 digest", release.Version)
		}
		if release.MinIngateVersion != "" && !semver.IsValid(release.MinIngateVersion) {
			return catalogRelease{}, false, fmt.Errorf("release %q has invalid minimum Ingate version", release.Version)
		}
		if !compatibleWithIngate(ingateVersion, release.MinIngateVersion) {
			continue
		}
		if selected.Version == "" || semver.Compare(canonicalVersion(release.Version), canonicalVersion(selected.Version)) > 0 {
			selected = release
		}
	}
	return selected, selected.Version != "", nil
}

func compatibleWithIngate(current, minimum string) bool {
	if minimum == "" || strings.Contains(current, "unknown") {
		return true
	}
	current = canonicalVersion(current)
	return semver.IsValid(current) && semver.Compare(current, minimum) >= 0
}

func canonicalVersion(value string) string {
	if value == "" || value[0] == 'v' {
		return value
	}
	return "v" + value
}

func validSHA256(value string) bool {
	digest, err := hex.DecodeString(value)
	return err == nil && len(digest) == sha256.Size
}

func validateCatalogURL(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return err
	}
	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("URL scheme must be http or https")
	}
	return nil
}
