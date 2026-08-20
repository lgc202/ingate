// Package server 提供控制台静态资源服务和管理 API 反向代理
package server

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// NewRouter 创建控制台静态资源与管理 API 转发路由
func NewRouter(adminAPIProxy http.Handler, consoleDir string, logger *slog.Logger) http.Handler {
	router := gin.New()
	router.Use(
		requestID(),
		cors(),
		recovery(logger),
	)

	router.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"code": http.StatusOK,
			"msg":  "ok",
			"data": gin.H{
				"status":    "ok",
				"requestID": ctx.GetString(requestid.Header),
			},
		})
	})
	router.Any("/api/*path", gin.WrapH(adminAPIProxy))

	mountConsole(router, consoleDir)
	return router
}

func mountConsole(router *gin.Engine, consoleDir string) {
	if consoleDir == "" {
		return
	}

	router.StaticFS("/assets", http.Dir(filepath.Join(consoleDir, "assets")))
	router.NoRoute(func(ctx *gin.Context) {
		if strings.HasPrefix(ctx.Request.URL.Path, "/api/") {
			ctx.JSON(http.StatusNotFound, gin.H{"message": "api not found"})
			return
		}
		ctx.File(filepath.Join(consoleDir, "index.html"))
	})
}
