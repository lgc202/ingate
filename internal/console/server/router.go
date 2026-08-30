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
		"/assets/",
		http.StripPrefix("/assets/", http.FileServer(http.Dir(filepath.Join(consoleDir, "assets")))),
	)
	indexFile := filepath.Join(consoleDir, "index.html")
	router.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, indexFile)
	})
	return router
}
