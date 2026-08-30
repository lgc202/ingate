// Package wasm 拉取、校验并缓存 Envoy 执行的 Wasm 模块。
package wasm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	containerv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	containertypes "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/tetratelabs/wazero"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	"github.com/lgc202/ingate/internal/controller/conf"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/httpurl"
	"github.com/lgc202/ingate/internal/pkg/version"
	"github.com/lgc202/ingate/internal/pkg/wasmconfig"
)

const (
	wasmFileName          = "plugin.wasm"
	wasmArtifactMediaType = "application/vnd.module.wasm.content.layer.v1+wasm"
	wasmMagic             = "\x00asm"
	// 官方 Envoy 当前支持 Proxy-Wasm ABI 0.2.0 和 0.2.1；供应商私有 ABI 不能进入配置发布链路。
	proxyWasmABI020  = "proxy_abi_version_0_2_0"
	proxyWasmABI021  = "proxy_abi_version_0_2_1"
	maxHTTPRedirects = 10
)

// Store 把远端模块转换为 Envoy 可从共享目录读取的内容寻址文件。
//
// Controller 只把校验后的本地副本交给 Envoy，避免 Envoy 直接访问外部仓库，
// 也确保 OCI manifest 摘要与最终 Wasm 二进制摘要分别按各自语义校验。
type Store struct {
	cacheDir      string
	pullTimeout   time.Duration
	maxModuleSize int64
	maxCacheSize  int64
	httpClient    *http.Client

	mu       sync.Mutex
	resolved map[compiler.ResourceGeneration]compiler.WasmModule
}

// NewStore 创建 Wasm 模块存储，并确保缓存目录可写。
func NewStore(config *conf.Data_Wasm) (*Store, error) {
	cacheDir, err := filepath.Abs(config.GetCacheDir())
	if err != nil {
		return nil, fmt.Errorf("resolve Wasm cache directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "sources"), 0o755); err != nil {
		return nil, fmt.Errorf("create Wasm cache directory: %w", err)
	}
	return &Store{
		cacheDir:      cacheDir,
		pullTimeout:   config.GetPullTimeout().AsDuration(),
		maxModuleSize: config.GetMaxModuleBytes(),
		maxCacheSize:  config.GetMaxCacheBytes(),
		httpClient:    &http.Client{CheckRedirect: checkHTTPRedirect},
		resolved:      make(map[compiler.ResourceGeneration]compiler.WasmModule),
	}, nil
}

// Resolve 返回已校验模块的本地文件路径和二进制 SHA256。
//
// Store 使用同一把锁串行保护远端拉取、缓存写入和淘汰决策，
// 确保新模块发布时不会破坏 Active 配置；
// 首版插件数量有限，不在这一边界额外引入并发下载调度。
func (s *Store) Resolve(ctx context.Context, plugin *gatewayv1.WasmPlugin) (compiler.WasmModule, error) {
	if !wasmconfig.IsValidArtifactURL(plugin.Spec.URL) {
		return compiler.WasmModule{}, errors.New("resolve Wasm module: invalid artifact URL")
	}
	if !wasmconfig.IsValidSHA256Digest(plugin.Spec.SHA256) {
		return compiler.WasmModule{}, errors.New("resolve Wasm module: invalid SHA256 digest")
	}
	switch plugin.Spec.PullPolicy {
	case gatewayv1.WasmPluginPullIfNotPresent, gatewayv1.WasmPluginPullAlways:
	default:
		return compiler.WasmModule{}, fmt.Errorf(
			"resolve Wasm module: unsupported pull policy %q",
			plugin.Spec.PullPolicy,
		)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	generation := compiler.ResourceGeneration{
		Kind:       gatewayv1.KindWasmPlugin,
		Name:       plugin.Name,
		UID:        plugin.UID,
		Generation: plugin.Generation,
	}
	if cached, exists := s.resolved[generation]; exists && s.moduleExists(cached.SHA256) {
		s.touchModule(cached.SHA256)
		return cached, nil
	}

	if plugin.Spec.PullPolicy == gatewayv1.WasmPluginPullIfNotPresent {
		if module, ok := s.cachedSource(ctx, plugin.Spec.URL, plugin.Spec.SHA256); ok {
			s.resolved[generation] = module
			return module, nil
		}
	}

	pullCtx, cancel := context.WithTimeout(ctx, s.pullTimeout)
	defer cancel()

	var (
		moduleBytes []byte
		err         error
	)
	if strings.HasPrefix(plugin.Spec.URL, "oci://") {
		moduleBytes, err = s.pullOCI(pullCtx, plugin.Spec.URL, plugin.Spec.SHA256)
	} else {
		moduleBytes, err = s.pullHTTP(pullCtx, plugin.Spec.URL, plugin.Spec.SHA256)
	}
	if err != nil {
		return compiler.WasmModule{}, err
	}
	if err := validateWasm(pullCtx, moduleBytes); err != nil {
		return compiler.WasmModule{}, err
	}

	moduleSHA := sha256Digest(moduleBytes)
	moduleAlreadyExists := s.moduleExists(moduleSHA)
	if err := s.reserveCache(moduleSHA, int64(len(moduleBytes))); err != nil {
		return compiler.WasmModule{}, err
	}
	if err := s.writeModule(moduleSHA, moduleBytes); err != nil {
		return compiler.WasmModule{}, err
	}
	if err := s.writeSourcePointer(plugin.Spec.URL, plugin.Spec.SHA256, moduleSHA); err != nil {
		if !moduleAlreadyExists {
			_ = os.Remove(s.modulePath(moduleSHA))
		}
		return compiler.WasmModule{}, err
	}

	module := compiler.WasmModule{Path: s.modulePath(moduleSHA), SHA256: moduleSHA}
	s.resolved[generation] = module
	return module, nil
}

// Retain 只保护本轮期望配置和 Delivery Active 配置仍引用的模块。
//
// 使用完整 generation 而不是资源 ID，
// 确保插件升级期间新旧模块可以共存到 Candidate ACK 或 NACK。
func (s *Store) Retain(generations []compiler.ResourceGeneration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	retained := make(map[compiler.ResourceGeneration]bool, len(generations))
	for _, generation := range generations {
		if generation.Kind == gatewayv1.KindWasmPlugin {
			retained[generation] = true
		}
	}
	for generation := range s.resolved {
		if !retained[generation] {
			delete(s.resolved, generation)
		}
	}
}

func (s *Store) pullHTTP(ctx context.Context, sourceURL, expectedSHA string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create Wasm download request: %w", err)
	}
	request.Header.Set("Accept", "application/wasm, application/octet-stream")
	request.Header.Set("User-Agent", "ingate-controller/"+version.String())
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download Wasm module: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download Wasm module: unexpected HTTP status %s", response.Status)
	}
	if response.ContentLength > s.maxModuleSize {
		return nil, fmt.Errorf(
			"wasm module declares %d bytes, exceeding maximum size %d bytes",
			response.ContentLength,
			s.maxModuleSize,
		)
	}
	moduleBytes, err := readModule(response.Body, s.maxModuleSize)
	if err != nil {
		return nil, err
	}
	if actual := sha256Digest(moduleBytes); actual != expectedSHA {
		return nil, fmt.Errorf("verify Wasm module SHA256: expected %s, got %s", expectedSHA, actual)
	}
	return moduleBytes, nil
}

func (s *Store) pullOCI(ctx context.Context, sourceURL, expectedManifestSHA string) ([]byte, error) {
	reference, err := name.ParseReference(
		strings.TrimPrefix(sourceURL, "oci://"),
		name.StrictValidation,
	)
	if err != nil {
		return nil, fmt.Errorf("parse OCI image reference: %w", err)
	}
	descriptor, err := remote.Get(reference, remote.WithAuth(authn.Anonymous), remote.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("fetch OCI manifest: %w", err)
	}
	manifestSHA := descriptor.Digest.Hex
	if manifestSHA != expectedManifestSHA {
		return nil, fmt.Errorf("verify OCI manifest SHA256: expected %s, got %s", expectedManifestSHA, manifestSHA)
	}
	image, err := descriptor.Image()
	if err != nil {
		return nil, fmt.Errorf("open OCI image: %w", err)
	}
	return s.extractModule(image)
}

func checkHTTPRedirect(request *http.Request, previous []*http.Request) error {
	if len(previous) >= maxHTTPRedirects {
		return fmt.Errorf("download Wasm module: exceeded %d redirects", maxHTTPRedirects)
	}
	if !httpurl.IsValid(request.URL.String()) {
		return errors.New("download Wasm module: redirect URL is invalid")
	}
	if len(previous) > 0 &&
		previous[len(previous)-1].URL.Scheme == "https" &&
		request.URL.Scheme != "https" {
		return errors.New("download Wasm module: redirect from HTTPS to HTTP is not allowed")
	}
	return nil
}

func (s *Store) extractModule(image containerv1.Image) ([]byte, error) {
	layers, err := image.Layers()
	if err != nil {
		return nil, fmt.Errorf("list OCI image layers: %w", err)
	}
	for _, layer := range layers {
		mediaType, err := layer.MediaType()
		if err != nil {
			return nil, fmt.Errorf("read OCI layer media type: %w", err)
		}
		if mediaType != containertypes.MediaType(wasmArtifactMediaType) {
			continue
		}
		reader, err := layer.Compressed()
		if err != nil {
			return nil, fmt.Errorf("open OCI Wasm layer: %w", err)
		}
		moduleBytes, readErr := readModule(reader, s.maxModuleSize)
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close OCI Wasm layer: %w", closeErr)
		}
		return moduleBytes, nil
	}
	if len(layers) == 0 {
		return nil, errors.New("OCI image does not contain a Wasm layer")
	}
	lastLayer := layers[len(layers)-1]
	mediaType, err := lastLayer.MediaType()
	if err != nil {
		return nil, fmt.Errorf("read OCI Wasm image layer media type: %w", err)
	}
	if mediaType != containertypes.OCILayer && mediaType != containertypes.DockerLayer {
		return nil, fmt.Errorf("OCI image does not contain a supported Wasm layer: %s", mediaType)
	}
	return s.extractWasmImageLayer(lastLayer)
}

func readModule(reader io.Reader, limit int64) ([]byte, error) {
	moduleBytes, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read Wasm module: %w", err)
	}
	if int64(len(moduleBytes)) > limit {
		return nil, fmt.Errorf("wasm module exceeds maximum size %d bytes", limit)
	}
	return moduleBytes, nil
}

func validateWasm(ctx context.Context, moduleBytes []byte) error {
	if len(moduleBytes) < 8 || string(moduleBytes[:len(wasmMagic)]) != wasmMagic {
		return errors.New("downloaded content is not a WebAssembly module")
	}
	wasmRuntime := wazero.NewRuntime(ctx)
	defer func() { _ = wasmRuntime.Close(context.WithoutCancel(ctx)) }()
	module, err := wasmRuntime.CompileModule(ctx, moduleBytes)
	if err != nil {
		return fmt.Errorf("compile WebAssembly module: %w", err)
	}
	exports := module.ExportedFunctions()
	if _, exists := exports[proxyWasmABI020]; exists {
		return nil
	}
	if _, exists := exports[proxyWasmABI021]; exists {
		return nil
	}
	return errors.New("WebAssembly module does not export a supported Proxy-Wasm ABI")
}

func sha256Digest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
