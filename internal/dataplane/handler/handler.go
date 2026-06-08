// Package handler 聚合 ingate-dataplane 的 HTTP handler
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	ratelimithandler "github.com/lgc202/ingate/internal/dataplane/handler/ratelimit"
	"github.com/lgc202/ingate/internal/dataplane/service"
)

// Handler 聚合数据面能力服务的 HTTP 入口
type Handler struct {
	RateLimit *ratelimithandler.Handler
}

// New 创建数据面 handler 聚合入口
func New(service *service.Service) *Handler {
	return &Handler{
		RateLimit: ratelimithandler.New(service.RateLimit),
	}
}

// Health 返回数据面服务健康状态
func (h *Handler) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
