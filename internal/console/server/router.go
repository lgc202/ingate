// Package server 提供控制台静态资源服务和管理 API 反向代理
package server

import (
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// NewRouter 创建控制台静态资源与管理 API 转发路由
func NewRouter(
	adminAPIProxy http.Handler,
	assistantProxy http.Handler,
	auth *SessionAuth,
	consoleDir string,
	logger *slog.Logger,
) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /healthz", func(response http.ResponseWriter, request *http.Request) {
		if err := writeJSON(response, http.StatusOK, map[string]any{
			"code": http.StatusOK,
			"msg":  "ok",
			"data": map[string]string{
				"status":    "ok",
				"requestID": request.Header.Get(requestid.Header),
			},
		}); err != nil {
			logger.Debug("write health response failed", "err", err)
		}
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
