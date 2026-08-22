// Package plugincatalog 加载并缓存官方插件目录
package plugincatalog

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"

	"github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const maxCatalogBytes = 1 << 20

//go:embed catalog.json
var fallbackCatalog []byte

type catalogState struct {
	items         []wasmplugin.CatalogItem
	specs         map[string]resource.WasmPluginSpec
	lastCheckedAt time.Time
	etag          string
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

// Catalog 为并发请求提供同一份不可变目录视图
// 远程刷新先完整解析和校验，再一次性替换当前视图，失败时不会污染已有目录
type Catalog struct {
	url             string
	refreshInterval time.Duration
	client          *http.Client
	logger          *slog.Logger

	refreshMu sync.Mutex
	stateMu   sync.RWMutex
	state     catalogState

	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
}

// NewCatalog 创建插件目录并加载随二进制发布的离线兜底数据
func NewCatalog(config *conf.Data, logger *slog.Logger) (*Catalog, error) {
	settings := config.GetPluginCatalog()
	remoteURL := strings.TrimSpace(settings.GetUrl())
	if remoteURL != "" {
		parsed, err := url.ParseRequestURI(remoteURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("parse plugin catalog URL %q", remoteURL)
		}
	}
	state, err := decodeCatalog(fallbackCatalog, version.String())
	if err != nil {
		return nil, fmt.Errorf("decode embedded plugin catalog: %w", err)
	}
	return &Catalog{
		url:             remoteURL,
		refreshInterval: settings.GetRefreshInterval().AsDuration(),
		client:          &http.Client{Timeout: settings.GetTimeout().AsDuration()},
		logger:          logger,
		state:           state,
	}, nil
}

// Snapshot 返回目录当前视图；返回切片副本避免调用方修改共享缓存
func (c *Catalog) Snapshot() wasmplugin.CatalogSnapshot {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return wasmplugin.CatalogSnapshot{
		Items:         append([]wasmplugin.CatalogItem(nil), c.state.items...),
		LastCheckedAt: c.state.lastCheckedAt,
	}
}

// PluginSpec 返回指定插件最新兼容版本的安装参数
func (c *Catalog) PluginSpec(packageName string) (resource.WasmPluginSpec, bool) {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	spec, ok := c.state.specs[packageName]
	return spec, ok
}

// Refresh 使用 HTTP ETag 检查远程目录并原子替换缓存
func (c *Catalog) Refresh(ctx context.Context) error {
	if c.url == "" {
		return nil
	}
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	c.stateMu.RLock()
	etag := c.state.etag
	c.stateMu.RUnlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return fmt.Errorf("create plugin catalog request: %w", err)
	}
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("fetch plugin catalog: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	now := time.Now()
	if response.StatusCode == http.StatusNotModified {
		c.stateMu.Lock()
		c.state.lastCheckedAt = now
		c.stateMu.Unlock()
		return nil
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch plugin catalog: unexpected HTTP status %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxCatalogBytes+1))
	if err != nil {
		return fmt.Errorf("read plugin catalog: %w", err)
	}
	if len(data) > maxCatalogBytes {
		return fmt.Errorf("read plugin catalog: response exceeds %d bytes", maxCatalogBytes)
	}
	state, err := decodeCatalog(data, version.String())
	if err != nil {
		return fmt.Errorf("decode remote plugin catalog: %w", err)
	}
	state.lastCheckedAt = now
	state.etag = response.Header.Get("ETag")

	c.stateMu.Lock()
	c.state = state
	c.stateMu.Unlock()
	return nil
}

// Start 启动目录的周期检查；生命周期由 Admin API 的 Kratos App 管理
func (c *Catalog) Start(ctx context.Context) {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.cancel != nil || c.url == "" {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	c.cancel = cancel
	c.done = done
	go c.run(runCtx, done)
}

// Stop 停止周期检查并等待后台任务退出
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
	c.refreshAndLog(ctx)
	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refreshAndLog(ctx)
		}
	}
}

func (c *Catalog) refreshAndLog(ctx context.Context) {
	if err := c.Refresh(ctx); err != nil && ctx.Err() == nil {
		c.logger.Warn("refresh plugin catalog failed", "error", err)
	}
}

func decodeCatalog(data []byte, ingateVersion string) (catalogState, error) {
	var file catalogFile
	if err := json.Unmarshal(data, &file); err != nil {
		return catalogState{}, err
	}
	state := catalogState{
		items: make([]wasmplugin.CatalogItem, 0, len(file.Plugins)),
		specs: make(map[string]resource.WasmPluginSpec, len(file.Plugins)),
	}
	for _, plugin := range file.Plugins {
		if err := addPlugin(&state, plugin, ingateVersion); err != nil {
			return catalogState{}, err
		}
	}
	return state, nil
}

func addPlugin(state *catalogState, plugin catalogPlugin, ingateVersion string) error {
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
		DisplayName: name,
		Package:     packageName,
		Version:     release.Version,
		URL:         fmt.Sprintf("oci://%s:v%s", strings.TrimSpace(release.Artifact.Repository), release.Version),
		SHA256:      strings.TrimPrefix(strings.TrimSpace(release.Artifact.SHA256), "sha256:"),
		PullPolicy:  resource.WasmPluginPullIfNotPresent,
	}
	return nil
}

func latestCompatibleRelease(
	releases []catalogRelease,
	ingateVersion string,
) (catalogRelease, bool, error) {
	var selected catalogRelease
	for _, release := range releases {
		release.Version = strings.TrimPrefix(strings.TrimSpace(release.Version), "v")
		release.MinIngateVersion = canonicalVersion(strings.TrimSpace(release.MinIngateVersion))
		release.Artifact.Repository = strings.TrimSpace(release.Artifact.Repository)
		if !semver.IsValid(canonicalVersion(release.Version)) || release.Artifact.Repository == "" {
			return catalogRelease{}, false, fmt.Errorf("release requires a semantic version and artifact repository")
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
