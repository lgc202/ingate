// Package ratelimit 处理限流 dataplane HTTP 请求
package ratelimit

import (
	"net/http"

	"github.com/gin-gonic/gin"

	ratelimitsvc "github.com/lgc202/ingate/internal/dataplane/service/ratelimit"
	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
)

// Handler 处理限流 dataplane 请求
type Handler struct {
	service *ratelimitsvc.Service
}

// New 创建限流 handler
func New(service *ratelimitsvc.Service) *Handler {
	return &Handler{service: service}
}

// Check 执行 Redis-backed 全局限流检查
func (h *Handler) Check(ctx *gin.Context) {
	request := dataplaneratelimit.CheckRequest{}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 单条 check 的语义错误由 service 写入对应 Result，保证插件可以按策略 fail-open / fail-close
	ctx.JSON(http.StatusOK, h.service.Check(ctx.Request.Context(), request))
}
