// Package handler 提供管理 API 的 HTTP 请求入口
package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/service"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// Handler 持有管理 API 各领域共用的服务与日志依赖
type Handler struct {
	services *service.Service
	logger   *slog.Logger
}

// New 创建管理 API Handler
func New(services *service.Service, logger *slog.Logger) *Handler {
	return &Handler{
		services: services,
		logger:   logger,
	}
}

// Health 返回服务健康状态
func (h *Handler) Health(ctx *gin.Context) {
	response.GinJSONResponse(ctx, http.StatusOK, "ok", gin.H{
		"status":    "ok",
		"requestID": ctx.GetString(requestid.Header),
	})
}
