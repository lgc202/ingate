package server

import (
	"net/http"
	"path/filepath"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// NewRouter 创建控制台静态资源与管理 API 转发路由。
func NewRouter(
	adminAPIProxy http.Handler,
	assistantProxy http.Handler,
	auth *SessionAuth,
	consoleDir string,
) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /healthz", func(response http.ResponseWriter, request *http.Request) {
		writeResponse(response, http.StatusOK, "ok", map[string]string{
			"status":    "ok",
			"requestID": request.Header.Get(requestid.Header),
		})
	})
	router.HandleFunc("/auth/session", auth.HandleSession)
	router.Handle("/api", auth.Protect(adminAPIProxy))
	router.Handle("/api/", auth.Protect(adminAPIProxy))
	router.Handle("/assistant/v1", auth.Protect(assistantProxy))
	router.Handle("/assistant/v1/", auth.Protect(assistantProxy))
	router.Handle(
		"GET /assets/",
		cacheImmutable(http.StripPrefix(
			"/assets/",
			http.FileServer(http.Dir(filepath.Join(consoleDir, "assets"))),
		)),
	)
	faviconFile := filepath.Join(consoleDir, "favicon.svg")
	router.HandleFunc("GET /favicon.svg", func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "public, max-age=3600")
		http.ServeFile(response, request, faviconFile)
	})
	indexFile := filepath.Join(consoleDir, "index.html")
	router.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.Header().Set("Allow", "GET, HEAD")
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		response.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(response, request, indexFile)
	})
	return router
}

func cacheImmutable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(response, request)
	})
}
