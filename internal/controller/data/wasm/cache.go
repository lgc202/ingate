package wasm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
)

type cacheFile struct {
	path    string
	size    int64
	modTime time.Time
}

func (s *Store) cachedSource(sourceURL, expectedSHA string) (compiler.WasmModule, bool) {
	pointerPath := s.sourcePointerPath(sourceURL, expectedSHA)
	pointer, err := os.ReadFile(pointerPath)
	if err != nil {
		return compiler.WasmModule{}, false
	}
	moduleSHA := strings.TrimSpace(string(pointer))
	if len(moduleSHA) != 64 {
		_ = os.Remove(pointerPath)
		return compiler.WasmModule{}, false
	}
	if _, err := hex.DecodeString(moduleSHA); err != nil {
		_ = os.Remove(pointerPath)
		return compiler.WasmModule{}, false
	}
	if !s.cachedModuleValid(moduleSHA) {
		_ = os.Remove(pointerPath)
		_ = os.Remove(s.modulePath(moduleSHA))
		return compiler.WasmModule{}, false
	}
	s.touchModule(moduleSHA)
	return compiler.WasmModule{Path: s.modulePath(moduleSHA), SHA256: moduleSHA}, true
}

// cachedModuleValid 在进程重启后首次复用磁盘文件时重新验证内容寻址约束
// 同一进程已经 Resolve 的模块由内存索引复用，不会在每次资源收敛时重复计算摘要
func (s *Store) cachedModuleValid(moduleSHA string) bool {
	file, err := os.Open(s.modulePath(moduleSHA))
	if err != nil {
		return false
	}
	binary, readErr := readModule(file, s.maxModuleSize)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || digest(binary) != moduleSHA {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.pullTimeout)
	defer cancel()
	return validateWasm(ctx, binary) == nil
}

func (s *Store) moduleExists(moduleSHA string) bool {
	info, err := os.Stat(s.modulePath(moduleSHA))
	return err == nil && info.Mode().IsRegular()
}

func (s *Store) touchModule(moduleSHA string) {
	now := time.Now()
	_ = os.Chtimes(s.modulePath(moduleSHA), now, now)
}

func (s *Store) writeModule(moduleSHA string, binary []byte) error {
	path := s.modulePath(moduleSHA)
	if s.moduleExists(moduleSHA) {
		s.touchModule(moduleSHA)
		return nil
	}
	return writeAtomic(path, binary, 0o644)
}

func (s *Store) writeSourcePointer(sourceURL, expectedSHA, moduleSHA string) error {
	return writeAtomic(s.sourcePointerPath(sourceURL, expectedSHA), []byte(moduleSHA+"\n"), 0o644)
}

func (s *Store) sourcePointerPath(sourceURL, expectedSHA string) string {
	sum := sha256.Sum256([]byte(sourceURL + "\x00" + expectedSHA))
	return filepath.Join(s.cacheDir, "sources", hex.EncodeToString(sum[:]))
}

func (s *Store) modulePath(moduleSHA string) string {
	return filepath.Join(s.cacheDir, moduleSHA+".wasm")
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".wasm-*")
	if err != nil {
		return fmt.Errorf("create temporary Wasm cache file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary Wasm cache file: %w", err)
	}
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set Wasm cache file permissions: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary Wasm cache file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish Wasm cache file: %w", err)
	}
	return nil
}

// reserveCache 只淘汰未被期望配置或 last-good xDS 引用的模块
//
// 容量不足时拒绝新模块，避免为了接纳 Candidate 而让 Active 配置的下载地址返回 404
func (s *Store) reserveCache(moduleSHA string, moduleSize int64) error {
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		return fmt.Errorf("read Wasm cache directory: %w", err)
	}
	files := make([]cacheFile, 0, len(entries))
	var total int64
	moduleExists := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect Wasm cache file %q: %w", entry.Name(), err)
		}
		files = append(files, cacheFile{
			path:    filepath.Join(s.cacheDir, entry.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		total += info.Size()
		if entry.Name() == moduleSHA+".wasm" {
			moduleExists = true
		}
	}
	if !moduleExists {
		total += moduleSize
	}
	slices.SortFunc(files, func(a, b cacheFile) int { return a.modTime.Compare(b.modTime) })
	protected := make(map[string]bool, len(s.resolved)+1)
	protected[s.modulePath(moduleSHA)] = true
	for _, resolved := range s.resolved {
		protected[s.modulePath(resolved.SHA256)] = true
	}
	for _, file := range files {
		if total <= s.maxCacheSize {
			break
		}
		if protected[file.path] {
			continue
		}
		if err := os.Remove(file.path); err != nil {
			return fmt.Errorf("remove expired Wasm cache file: %w", err)
		}
		total -= file.size
	}
	if total > s.maxCacheSize {
		return fmt.Errorf("wasm cache cannot reserve %d bytes within maximum size %d bytes", moduleSize, s.maxCacheSize)
	}
	return nil
}
