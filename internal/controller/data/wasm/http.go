package wasm

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var moduleRequestPattern = regexp.MustCompile(`^[a-f0-9]{64}\.wasm$`)

// ServeHTTP 仅提供内容寻址的已校验模块，不暴露缓存目录或原始下载地址
func (s *Store) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(request.URL.Path, modulePathPrefix)
	if request.URL.Path == name || !moduleRequestPattern.MatchString(name) {
		http.NotFound(response, request)
		return
	}
	file, err := os.Open(filepath.Join(s.cacheDir, name))
	if err != nil {
		http.NotFound(response, request)
		return
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Content-Type", "application/wasm")
	response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(response, request, name, info.ModTime(), file)
}
